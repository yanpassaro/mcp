package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	mobyclient "github.com/moby/moby/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"ntdsk.com/mcp/docker/internal/docker"
)

const (
	DEFAULT_TAIL        = 200
	MAX_TAIL            = 10000
	MAX_LAST            = 1000
	MAX_LIMIT           = 5000
	EXEC_POLL_INTERVAL  = 100 * time.Millisecond
	EXEC_POLL_ATTEMPTS  = 10
)

type Server struct {
	client *docker.Client
}

func New(client *docker.Client) *Server {
	return &Server{client: client}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_get",
		Description: "List Docker resources. 'resource': containers, images, volumes, networks, info, stats or disk. Read-only.",
	}, s.get)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_inspect",
		Description: "Inspect JSON for a container, image, volume or network. 'resource' selects the type (container default); container Environment variables (Env) are removed.",
	}, s.inspect)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_run",
		Description: "Create and start a detached container. Image pull policy (missing/always/never), optional platform and timeout. No environment variables are accepted.",
	}, s.run)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_control",
		Description: "Act on a container: start, stop, restart, pause, unpause, kill, logs or exec. Use action=remove with resource=container|image|volume|network to remove resources.",
	}, s.control)
}

func (s *Server) get(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
	ctx, cancel := s.client.WithTimeout(ctx)
	defer cancel()

	resource := strings.ToLower(strings.TrimSpace(in.Resource))
	switch resource {
	case "containers", "container", "ps":
		return s.getContainers(ctx, in)
	case "images", "image":
		return s.getImages(ctx, in)
	case "volumes", "volume":
		return s.getVolumes(ctx, in)
	case "networks", "network":
		return s.getNetworks(ctx, in)
	case "info":
		return s.getInfo(ctx)
	case "stats":
		return s.getStats(ctx, in)
	case "disk", "df", "disk_usage":
		return s.getDisk(ctx, in)
	default:
		return nil, nil, errors.New("'resource' inválido: use containers, images, volumes, networks, info, stats ou disk")
	}
}

func (s *Server) getContainers(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	filters, err := parseFilters(in.Filter)
	if err != nil {
		return nil, nil, err
	}
	result, err := s.client.API().ContainerList(ctx, mobyclient.ContainerListOptions{
		All:     !in.RunningOnly,
		Limit:   clampLast(in.Last),
		Filters: filters,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list containers: %w", err)
	}
	items := limitRows(result.Items, clampLimit(in.Limit))
	return textResult(limitText(formatContainers(items), len(result.Items), len(items)))
}

func (s *Server) getImages(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	filters, err := parseFilters(in.Filter)
	if err != nil {
		return nil, nil, err
	}
	result, err := s.client.API().ImageList(ctx, mobyclient.ImageListOptions{All: in.All, Filters: filters})
	if err != nil {
		return nil, nil, fmt.Errorf("list images: %w", err)
	}
	items := limitRows(result.Items, clampLimit(in.Limit))
	return textResult(limitText(formatImages(items), len(result.Items), len(items)))
}

func (s *Server) getVolumes(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	filters, err := parseFilters(in.Filter)
	if err != nil {
		return nil, nil, err
	}
	result, err := s.client.API().VolumeList(ctx, mobyclient.VolumeListOptions{Filters: filters})
	if err != nil {
		return nil, nil, fmt.Errorf("list volumes: %w", err)
	}
	items := limitRows(result.Items, clampLimit(in.Limit))
	return textResult(limitText(formatVolumes(items), len(result.Items), len(items)))
}

func (s *Server) getNetworks(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	filters, err := parseFilters(in.Filter)
	if err != nil {
		return nil, nil, err
	}
	result, err := s.client.API().NetworkList(ctx, mobyclient.NetworkListOptions{Filters: filters})
	if err != nil {
		return nil, nil, fmt.Errorf("list networks: %w", err)
	}
	items := limitRows(result.Items, clampLimit(in.Limit))
	return textResult(limitText(formatNetworks(items), len(result.Items), len(items)))
}

func (s *Server) getInfo(ctx context.Context) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().Info(ctx, mobyclient.InfoOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("docker info: %w", err)
	}
	return textResult(formatInfo(result.Info))
}

func (s *Server) getStats(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	api := s.client.API()
	targets := []string{}
	requested := strings.TrimSpace(in.Container)
	if requested != "" {
		targets = append(targets, requested)
	}
	if requested == "" {
		list, err := api.ContainerList(ctx, mobyclient.ContainerListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("list containers: %w", err)
		}
		for _, item := range list.Items {
			targets = append(targets, item.ID)
		}
	}
	targets = limitRows(targets, clampLimit(in.Limit))
	stats := make([]container.StatsResponse, 0, len(targets))
	for _, target := range targets {
		result, err := api.ContainerStats(ctx, target, mobyclient.ContainerStatsOptions{IncludePreviousSample: true})
		if err != nil {
			return nil, nil, fmt.Errorf("stats %s: %w", target, err)
		}
		var stat container.StatsResponse
		decodeErr := json.NewDecoder(result.Body).Decode(&stat)
		closeErr := result.Body.Close()
		if decodeErr != nil {
			return nil, nil, fmt.Errorf("decode stats %s: %w", target, decodeErr)
		}
		if closeErr != nil {
			return nil, nil, fmt.Errorf("close stats %s: %w", target, closeErr)
		}
		stats = append(stats, stat)
	}
	return textResult(formatStats(stats))
}

func (s *Server) getDisk(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().DiskUsage(ctx, mobyclient.DiskUsageOptions{
		Containers: true,
		Images:     true,
		Volumes:    true,
		BuildCache: true,
		Verbose:    in.Verbose,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("docker disk usage: %w", err)
	}
	return textResult(formatDiskUsage(result))
}

func (s *Server) inspect(ctx context.Context, _ *mcp.CallToolRequest, in inspectInput) (*mcp.CallToolResult, any, error) {
	target := strings.TrimSpace(in.Target)
	if target == "" {
		target = strings.TrimSpace(in.Container)
	}
	if target == "" {
		return nil, nil, errors.New("target é obrigatório")
	}
	ctx, cancel := s.client.WithTimeout(ctx)
	defer cancel()

	api := s.client.API()
	resource := strings.ToLower(strings.TrimSpace(in.Resource))
	switch resource {
	case "", "container", "containers":
		result, err := api.ContainerInspect(ctx, target, mobyclient.ContainerInspectOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("inspect container %s: %w", target, err)
		}
		value, err := decodeInspect([]byte(result.Raw))
		if err != nil {
			return nil, nil, err
		}
		return textResult(formatInspect("container", target, redactEnv(value)))
	case "image", "images":
		var raw bytes.Buffer
		result, err := api.ImageInspect(ctx, target, mobyclient.ImageInspectWithRawResponse(&raw))
		if err != nil {
			return nil, nil, fmt.Errorf("inspect image %s: %w", target, err)
		}
		if raw.Len() == 0 {
			return textResult(formatInspect("image", target, result))
		}
		value, err := decodeInspect(raw.Bytes())
		if err != nil {
			return nil, nil, err
		}
		return textResult(formatInspect("image", target, value))
	case "volume", "volumes":
		result, err := api.VolumeInspect(ctx, target, mobyclient.VolumeInspectOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("inspect volume %s: %w", target, err)
		}
		value, err := decodeInspect([]byte(result.Raw))
		if err != nil {
			return nil, nil, err
		}
		return textResult(formatInspect("volume", target, value))
	case "network", "networks":
		result, err := api.NetworkInspect(ctx, target, mobyclient.NetworkInspectOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("inspect network %s: %w", target, err)
		}
		value, err := decodeInspect([]byte(result.Raw))
		if err != nil {
			return nil, nil, err
		}
		return textResult(formatInspect("network", target, value))
	default:
		return nil, nil, errors.New("'resource' inválido: use container, image, volume ou network")
	}
}

func decodeInspect(raw []byte) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode docker inspect: %w", err)
	}
	return value, nil
}

func (s *Server) logs(ctx context.Context, containerID string, in controlInput) (*mcp.CallToolResult, any, error) {
	info, err := s.client.API().ContainerInspect(ctx, containerID, mobyclient.ContainerInspectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s: %w", containerID, err)
	}
	tty, err := inspectTTY(info.Raw)
	if err != nil {
		return nil, nil, err
	}
	tail := clampTail(in.Tail)
	result, err := s.client.API().ContainerLogs(ctx, containerID, mobyclient.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      strings.TrimSpace(in.Since),
		Until:      strings.TrimSpace(in.Until),
		Timestamps: in.Timestamps,
		Tail:       strconv.Itoa(tail),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("logs %s: %w", containerID, err)
	}

	var out bytes.Buffer
	var readErr error
	if tty {
		_, readErr = io.Copy(&out, result)
	}
	if !tty {
		var stderr bytes.Buffer
		_, readErr = stdcopy.StdCopy(&out, &stderr, result)
		if stderr.Len() > 0 {
			out.WriteString("\n")
			out.Write(stderr.Bytes())
		}
	}
	closeErr := result.Close()
	if readErr != nil {
		return nil, nil, fmt.Errorf("read logs %s: %w", containerID, readErr)
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("close logs %s: %w", containerID, closeErr)
	}
	return textResult(formatLogs(containerID, tail, out.String()))
}

func (s *Server) run(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
	rawImage := strings.TrimSpace(in.Image)
	if rawImage == "" {
		return nil, nil, errors.New("image é obrigatório")
	}
	imageName := normalizeImage(rawImage)
	policy, err := pullPolicy(in.Pull)
	if err != nil {
		return nil, nil, err
	}
	platform, err := parsePlatform(in.Platform)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := s.client.WithTimeoutSeconds(ctx, in.TimeoutSeconds)
	defer cancel()
	pulled, err := s.ensureImage(ctx, imageName, policy, platform)
	if err != nil {
		return nil, nil, err
	}

	natExposed, natBindings, err := nat.ParsePortSpecs(in.Ports)
	if err != nil {
		return nil, nil, fmt.Errorf("portas inválidas: %w", err)
	}
	ports := network.PortMap{}
	for natPort, bindings := range natBindings {
		port, err := network.ParsePort(string(natPort))
		if err != nil {
			return nil, nil, fmt.Errorf("porta inválida %q: %w", natPort, err)
		}
		converted := make([]network.PortBinding, 0, len(bindings))
		for _, binding := range bindings {
			hostIP := netip.Addr{}
			if strings.TrimSpace(binding.HostIP) != "" {
				parsed, err := netip.ParseAddr(binding.HostIP)
				if err != nil {
					return nil, nil, fmt.Errorf("host IP inválido %q: %w", binding.HostIP, err)
				}
				hostIP = parsed
			}
			converted = append(converted, network.PortBinding{HostIP: hostIP, HostPort: binding.HostPort})
		}
		ports[port] = converted
	}
	exposed := network.PortSet{}
	for natPort := range natExposed {
		port, err := network.ParsePort(string(natPort))
		if err != nil {
			return nil, nil, fmt.Errorf("porta inválida %q: %w", natPort, err)
		}
		exposed[port] = struct{}{}
	}
	name := strings.TrimSpace(in.Name)
	config := &container.Config{Image: imageName, Cmd: in.Command, ExposedPorts: exposed}
	host := &container.HostConfig{
		PortBindings: ports,
		Binds:        in.Volumes,
		AutoRemove:   in.AutoRemove,
	}
	networkName := strings.TrimSpace(in.Network)
	if networkName != "" {
		host.NetworkMode = container.NetworkMode(networkName)
	}
	created, err := s.client.API().ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config:     config,
		HostConfig: host,
		Name:       name,
		Platform:   platform,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create container: %w", err)
	}
	_, err = s.client.API().ContainerStart(ctx, created.ID, mobyclient.ContainerStartOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("container %s criado mas não iniciado: %w", created.ID, err)
	}
	return textResult(formatRun(name, imageName, created, pulled))
}

func (s *Server) control(ctx context.Context, _ *mcp.CallToolRequest, in controlInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	target := strings.TrimSpace(in.Target)
	if target == "" {
		target = strings.TrimSpace(in.Container)
	}
	if target == "" {
		return nil, nil, errors.New("target é obrigatório")
	}
	ctx, cancel := s.client.WithTimeoutSeconds(ctx, in.TimeoutSeconds)
	defer cancel()

	api := s.client.API()
	switch action {
	case "start":
		_, err := api.ContainerStart(ctx, target, mobyclient.ContainerStartOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("start %s: %w", target, err)
		}
		return textResult(formatControl("start", target, ""))
	case "stop":
		options := mobyclient.ContainerStopOptions{Signal: strings.TrimSpace(in.Signal)}
		if in.TimeoutSeconds > 0 {
			timeout := in.TimeoutSeconds
			options.Timeout = &timeout
		}
		_, err := api.ContainerStop(ctx, target, options)
		if err != nil {
			return nil, nil, fmt.Errorf("stop %s: %w", target, err)
		}
		return textResult(formatControl("stop", target, ""))
	case "restart":
		options := mobyclient.ContainerRestartOptions{Signal: strings.TrimSpace(in.Signal)}
		if in.TimeoutSeconds > 0 {
			timeout := in.TimeoutSeconds
			options.Timeout = &timeout
		}
		_, err := api.ContainerRestart(ctx, target, options)
		if err != nil {
			return nil, nil, fmt.Errorf("restart %s: %w", target, err)
		}
		return textResult(formatControl("restart", target, ""))
	case "pause":
		_, err := api.ContainerPause(ctx, target, mobyclient.ContainerPauseOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("pause %s: %w", target, err)
		}
		return textResult(formatControl("pause", target, ""))
	case "unpause":
		_, err := api.ContainerUnpause(ctx, target, mobyclient.ContainerUnpauseOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("unpause %s: %w", target, err)
		}
		return textResult(formatControl("unpause", target, ""))
	case "kill":
		signal := strings.TrimSpace(in.Signal)
		if signal == "" {
			signal = "SIGKILL"
		}
		_, err := api.ContainerKill(ctx, target, mobyclient.ContainerKillOptions{Signal: signal})
		if err != nil {
			return nil, nil, fmt.Errorf("kill %s: %w", target, err)
		}
		return textResult(formatControl("kill", target, signal))
	case "remove":
		return s.removeResource(ctx, target, in.Resource, in.Force, in.Volumes)
	case "logs":
		return s.logs(ctx, target, in)
	case "exec":
		return s.execCommand(ctx, target, in)
	default:
		return nil, nil, errors.New("'action' inválido: use start, stop, restart, pause, unpause, kill, logs, exec ou remove")
	}
}

func (s *Server) removeResource(ctx context.Context, target, resource string, force, volumes bool) (*mcp.CallToolResult, any, error) {
	api := s.client.API()
	resource = strings.ToLower(strings.TrimSpace(resource))
	switch resource {
	case "", "container", "containers":
		_, err := api.ContainerRemove(ctx, target, mobyclient.ContainerRemoveOptions{
			Force:         force,
			RemoveVolumes: volumes,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("remove container %s: %w", target, err)
		}
		return textResult(formatRemove("container", target))
	case "image", "images":
		_, err := api.ImageRemove(ctx, target, mobyclient.ImageRemoveOptions{
			Force:         force,
			PruneChildren: true,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("remove image %s: %w", target, err)
		}
		return textResult(formatRemove("image", target))
	case "volume", "volumes":
		_, err := api.VolumeRemove(ctx, target, mobyclient.VolumeRemoveOptions{Force: force})
		if err != nil {
			return nil, nil, fmt.Errorf("remove volume %s: %w", target, err)
		}
		return textResult(formatRemove("volume", target))
	case "network", "networks":
		_, err := api.NetworkRemove(ctx, target, mobyclient.NetworkRemoveOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("remove network %s: %w", target, err)
		}
		return textResult(formatRemove("network", target))
	default:
		return nil, nil, errors.New("'resource' inválido: use container, image, volume ou network")
	}
}

func (s *Server) execCommand(ctx context.Context, target string, in controlInput) (*mcp.CallToolResult, any, error) {
	if len(in.Command) == 0 {
		return nil, nil, errors.New("command é obrigatório para exec")
	}
	command := in.Command
	api := s.client.API()
	created, err := api.ExecCreate(ctx, target, mobyclient.ExecCreateOptions{
		User:         strings.TrimSpace(in.User),
		WorkingDir:   strings.TrimSpace(in.Workdir),
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("exec create %s: %w", target, err)
	}
	attach, err := api.ExecAttach(ctx, created.ID, mobyclient.ExecAttachOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("exec attach %s: %w", target, err)
	}

	stream := make(chan execStream, 1)
	go func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		_, copyErr := stdcopy.StdCopy(&stdout, &stderr, attach.Reader)
		attach.Close()
		stream <- execStream{stdout: stdout.String(), stderr: stderr.String(), err: copyErr}
	}()

	var out execStream
	select {
	case out = <-stream:
	case <-ctx.Done():
		attach.Close()
		return nil, nil, fmt.Errorf("exec %s: %w", target, ctx.Err())
	}
	if out.err != nil {
		return nil, nil, fmt.Errorf("exec read %s: %w", target, out.err)
	}

	info, err := s.waitExec(ctx, created.ID)
	if err != nil {
		return nil, nil, err
	}
	return textResult(formatExec(target, command, info.ExitCode, out.stdout, out.stderr))
}

func (s *Server) waitExec(ctx context.Context, execID string) (mobyclient.ExecInspectResult, error) {
	api := s.client.API()
	for i := 0; i < EXEC_POLL_ATTEMPTS; i++ {
		info, err := api.ExecInspect(ctx, execID, mobyclient.ExecInspectOptions{})
		if err != nil {
			return mobyclient.ExecInspectResult{}, fmt.Errorf("exec inspect %s: %w", execID, err)
		}
		if !info.Running {
			return info, nil
		}
		time.Sleep(EXEC_POLL_INTERVAL)
	}
	return mobyclient.ExecInspectResult{}, fmt.Errorf("exec %s: processo ainda em execução", execID)
}

func inspectTTY(raw json.RawMessage) (bool, error) {
	var meta struct {
		Config struct {
			Tty bool `json:"Tty"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return false, fmt.Errorf("decode container inspect: %w", err)
	}
	return meta.Config.Tty, nil
}

func pullPolicy(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "missing":
		return "missing", nil
	case "always":
		return "always", nil
	case "never":
		return "never", nil
	default:
		return "", fmt.Errorf("pull inválido: use missing, always ou never")
	}
}

func parsePlatform(value string) (*ocispec.Platform, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("platform inválida %q: use os/arch[/variant]", value)
	}
	if len(parts) > 3 {
		return nil, fmt.Errorf("platform inválida %q: use os/arch[/variant]", value)
	}
	platform := &ocispec.Platform{OS: parts[0], Architecture: parts[1]}
	if len(parts) == 3 {
		platform.Variant = parts[2]
	}
	if platform.OS == "" {
		return nil, fmt.Errorf("platform inválida %q: OS vazio", value)
	}
	if platform.Architecture == "" {
		return nil, fmt.Errorf("platform inválida %q: arquitetura vazia", value)
	}
	return platform, nil
}

func normalizeImage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.Contains(value, "@") {
		return value
	}
	slash := strings.LastIndex(value, "/")
	colon := strings.LastIndex(value, ":")
	if colon > slash {
		return value
	}
	return fmt.Sprintf("%s:latest", value)
}

func imageExists(ctx context.Context, api *mobyclient.Client, image string) (bool, error) {
	_, err := api.ImageInspect(ctx, image)
	if err == nil {
		return true, nil
	}
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("inspect image %s: %w", image, err)
}

func (s *Server) ensureImage(ctx context.Context, image, policy string, platform *ocispec.Platform) (bool, error) {
	exists, err := imageExists(ctx, s.client.API(), image)
	if err != nil {
		return false, err
	}
	if policy == "never" {
		if !exists {
			return false, fmt.Errorf("imagem %q não encontrada e pull=never", image)
		}
		return false, nil
	}
	if policy == "always" {
		if err := s.pullImage(ctx, image, platform); err != nil {
			return false, err
		}
		return true, nil
	}
	if exists {
		return false, nil
	}
	if err := s.pullImage(ctx, image, platform); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Server) pullImage(ctx context.Context, image string, platform *ocispec.Platform) error {
	options := mobyclient.ImagePullOptions{RegistryAuth: s.client.RegistryAuth()}
	if platform != nil {
		options.Platforms = []ocispec.Platform{*platform}
	}
	response, err := s.client.API().ImagePull(ctx, image, options)
	if err != nil {
		return fmt.Errorf("pull %s: %w", image, err)
	}
	waitErr := response.Wait(ctx)
	closeErr := response.Close()
	if waitErr != nil {
		return fmt.Errorf("pull %s: %w", image, waitErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close pull %s: %w", image, closeErr)
	}
	return nil
}

func clampTail(n int) int {
	if n <= 0 {
		return DEFAULT_TAIL
	}
	if n > MAX_TAIL {
		return MAX_TAIL
	}
	return n
}

func clampLast(n int) int {
	if n <= 0 {
		return 0
	}
	if n > MAX_LAST {
		return MAX_LAST
	}
	return n
}

func parseFilters(value string) (mobyclient.Filters, error) {
	filters := mobyclient.Filters{}
	value = strings.TrimSpace(value)
	if value == "" {
		return filters, nil
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("filtro inválido %q: use chave=valor", part)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			return nil, fmt.Errorf("filtro inválido %q: chave vazia", part)
		}
		filters.Add(key, val)
	}
	return filters, nil
}

func clampLimit(n int) int {
	if n <= 0 {
		return 0
	}
	if n > MAX_LIMIT {
		return MAX_LIMIT
	}
	return n
}

func limitRows[T any](rows []T, limit int) []T {
	if limit <= 0 {
		return rows
	}
	if len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func limitText(text string, total, shown int) string {
	if shown >= total {
		return text
	}
	return fmt.Sprintf("%s\n\nMostrando %d de %d registros. Use limit para ajustar.", text, shown, total)
}

type getInput struct {
	Resource    string `json:"resource" jsonschema:"What to get: containers, images, volumes, networks, info, stats or disk (required)."`
	RunningOnly bool   `json:"runningOnly,omitempty" jsonschema:"For containers: if true, shows only running containers; default includes stopped."`
	All         bool   `json:"all,omitempty" jsonschema:"For images: include intermediate images."`
	Last        int    `json:"last,omitempty" jsonschema:"For containers: last N created containers."`
	Filter      string `json:"filter,omitempty" jsonschema:"Docker filter expressions, comma-separated key=value (e.g. status=running,name=api)."`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum rows returned; 0 = all."`
	Verbose     bool   `json:"verbose,omitempty" jsonschema:"For disk: detailed per-object usage."`
	Container   string `json:"container,omitempty" jsonschema:"For stats: container name or ID; default all running containers."`
}

type inspectInput struct {
	Resource  string `json:"resource,omitempty" jsonschema:"What to inspect: container (default), image, volume or network."`
	Target    string `json:"target,omitempty" jsonschema:"Container, image, volume or network name/ID (required)."`
	Container string `json:"container,omitempty" jsonschema:"Legacy alias for target (container name/ID)."`
}

type runInput struct {
	Image          string   `json:"image" jsonschema:"Image to run (required)."`
	Name           string   `json:"name,omitempty" jsonschema:"Container name (optional)."`
	Command        []string `json:"command,omitempty" jsonschema:"Command and arguments after the image (optional)."`
	Ports          []string `json:"ports,omitempty" jsonschema:"Port mappings, e.g. [\"8080:80\"]."`
	Volumes        []string `json:"volumes,omitempty" jsonschema:"Volume mappings, e.g. [\"C:/data:/data\"]."`
	Network        string   `json:"network,omitempty" jsonschema:"Network name (optional)."`
	AutoRemove     bool     `json:"autoRemove,omitempty" jsonschema:"If true, removes the container when it stops."`
	Pull           string   `json:"pull,omitempty" jsonschema:"Image pull policy: missing (default), always or never."`
	Platform       string   `json:"platform,omitempty" jsonschema:"Optional platform, e.g. linux/amd64."`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty" jsonschema:"Optional timeout override for pull/create/start, in seconds."`
}

type controlInput struct {
	Action         string   `json:"action" jsonschema:"Action: start, stop, restart, pause, unpause, kill, logs, exec or remove (required)."`
	Target         string   `json:"target,omitempty" jsonschema:"Container name/ID; for remove, container/image/volume/network name/ID (required)."`
	Container      string   `json:"container,omitempty" jsonschema:"Legacy alias for target (container name/ID)."`
	Resource       string   `json:"resource,omitempty" jsonschema:"For remove: container (default), image, volume or network."`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty" jsonschema:"Operation timeout in seconds; for stop/restart, also the graceful timeout."`
	Signal         string   `json:"signal,omitempty" jsonschema:"For stop/restart/kill: signal name."`
	Force          bool     `json:"force,omitempty" jsonschema:"For remove: force removal."`
	Volumes        bool     `json:"volumes,omitempty" jsonschema:"For remove container: remove anonymous volumes."`
	Command        []string `json:"command,omitempty" jsonschema:"For exec: command and arguments, e.g. [\"sh\",\"-lc\",\"ls -la\"]."`
	User           string   `json:"user,omitempty" jsonschema:"For exec: user (name or UID[:GID]) to run the command."`
	Workdir        string   `json:"workdir,omitempty" jsonschema:"For exec: working directory inside the container."`
	Tail           int      `json:"tail,omitempty" jsonschema:"For logs: number of lines from the end (default 200, max 10000)."`
	Since          string   `json:"since,omitempty" jsonschema:"For logs: show logs since a timestamp or relative duration (e.g. 10m)."`
	Until          string   `json:"until,omitempty" jsonschema:"For logs: show logs before a timestamp or relative duration."`
	Timestamps     bool     `json:"timestamps,omitempty" jsonschema:"For logs: prefix each line with its timestamp."`
}

type execStream struct {
	stdout string
	stderr string
	err    error
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
