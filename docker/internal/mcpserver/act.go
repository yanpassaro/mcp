package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) act(ctx context.Context, _ *mcp.CallToolRequest, in actInput) (*mcp.CallToolResult, any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Action))
	d := in.Data
	switch action {
	case "run":
		return s.run(ctx, nil, runInput{
			Image:          d.Image,
			Name:           d.Name,
			Command:        d.Command,
			Ports:          d.Ports,
			Volumes:        d.Volumes,
			Network:        d.Network,
			AutoRemove:     d.AutoRemove,
			Env:            d.Env,
			EnvFile:        d.EnvFile,
			Restart:        d.Restart,
			Memory:         d.Memory,
			CPUs:           d.CPUs,
			Workdir:        d.Workdir,
			User:           d.User,
			Pull:           d.Pull,
			Platform:       d.Platform,
			TimeoutSeconds: d.Timeout,
			BuildContext:   d.Context,
			Dockerfile:     d.Dockerfile,
			BuildArgs:      d.BuildArgs,
			BuildPull:      d.BuildPull,
			NoCache:        d.NoCache,
			Target:         d.BuildTarget,
		})
	case "up":
		return s.run(ctx, nil, runInput{Compose: d.File, Project: d.Project, Services: d.Services, Build: d.Build, TimeoutSeconds: d.Timeout})
	case "down":
		return s.control(ctx, nil, controlInput{Action: "down", File: d.File, Project: d.Project, Volumes: d.RemoveVolumes})
	case "remove":
		return s.control(ctx, nil, controlInput{Action: "remove", Target: d.Name, Resource: d.Resource, Force: d.Force, Volumes: d.RemoveVolumes})
	case "exec":
		return s.control(ctx, nil, controlInput{Action: "exec", Target: d.Name, Command: d.Command, User: d.User, Workdir: d.Workdir, TimeoutSeconds: d.Timeout})
	case "start", "stop", "restart", "pause", "unpause", "kill":
		return s.control(ctx, nil, controlInput{Action: action, Target: d.Name, Signal: d.Signal, TimeoutSeconds: d.Timeout})
	case "":
		return nil, nil, errors.New("'action' é obrigatório")
	default:
		return nil, nil, errors.New("'action' inválido: run, up, down, start, stop, restart, pause, unpause, kill, remove, exec")
	}
}

type actInput struct {
	Action string  `json:"action" jsonschema:"run, up, down, start, stop, restart, pause, unpause, kill, remove or exec (required)."`
	Data   actData `json:"data,omitempty" jsonschema:"Options for the chosen action."`
}

type actData struct {
	Name           string   `json:"name,omitempty" jsonschema:"Container name/ID; resource name for remove."`
	Image          string   `json:"image,omitempty" jsonschema:"Image to run."`
	Command        []string `json:"command,omitempty" jsonschema:"Command and args (run, exec)."`
	Ports          []string `json:"ports,omitempty" jsonschema:"Port mappings, e.g. [\"8080:80\"]."`
	Volumes        []string `json:"volumes,omitempty" jsonschema:"Volume mappings, e.g. [\"C:/data:/data\"]."`
	Network        string   `json:"network,omitempty" jsonschema:"Network name."`
	AutoRemove     bool     `json:"autoRemove,omitempty" jsonschema:"Remove the container when it stops."`
	Env            []string `json:"env,omitempty" jsonschema:"Env vars as KEY=VALUE; values are never echoed back."`
	EnvFile        string   `json:"envFile,omitempty" jsonschema:"Env file on this machine; values are never echoed back."`
	Restart        string   `json:"restart,omitempty" jsonschema:"no, always, unless-stopped, on-failure[:n]."`
	Memory         string   `json:"memory,omitempty" jsonschema:"Memory limit, e.g. 512m."`
	CPUs           float64  `json:"cpus,omitempty" jsonschema:"CPU limit in cores, e.g. 1.5."`
	Workdir        string   `json:"workdir,omitempty" jsonschema:"Working directory (run, exec)."`
	User           string   `json:"user,omitempty" jsonschema:"User name or UID[:GID] (run, exec)."`
	Pull           string   `json:"pull,omitempty" jsonschema:"missing, always or never."`
	Platform       string   `json:"platform,omitempty" jsonschema:"e.g. linux/amd64."`
	Context        string   `json:"context,omitempty" jsonschema:"Build context directory; builds the image before running."`
	Dockerfile     string   `json:"dockerfile,omitempty" jsonschema:"Dockerfile name or path."`
	BuildArgs      []string `json:"buildArgs,omitempty" jsonschema:"Build args as KEY=VALUE."`
	BuildPull      bool     `json:"buildPull,omitempty" jsonschema:"Always pull newer base images."`
	NoCache        bool     `json:"noCache,omitempty" jsonschema:"Do not use the build cache."`
	BuildTarget    string   `json:"buildTarget,omitempty" jsonschema:"Build stage to target."`
	File           string   `json:"file,omitempty" jsonschema:"Compose file path (up, down)."`
	Project        string   `json:"project,omitempty" jsonschema:"Compose project name."`
	Services       []string `json:"services,omitempty" jsonschema:"Compose services; empty = all."`
	Build          bool     `json:"build,omitempty" jsonschema:"Compose: build images before starting."`
	Resource       string   `json:"resource,omitempty" jsonschema:"Remove type: container, image, volume or network."`
	Signal         string   `json:"signal,omitempty" jsonschema:"Signal for stop/restart/kill."`
	Force          bool     `json:"force,omitempty" jsonschema:"Remove: force."`
	RemoveVolumes  bool     `json:"removeVolumes,omitempty" jsonschema:"remove/down: also remove volumes."`
	Timeout        int      `json:"timeout,omitempty" jsonschema:"Timeout in seconds."`
}
