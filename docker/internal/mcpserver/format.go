package mcpserver

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	mobyclient "github.com/moby/moby/client"
)

const (
	maxCellBytes    = 80
	maxLogBytes     = 128 * 1024
	maxInspectBytes = 128 * 1024
	maxDiskBytes    = 128 * 1024
)

func table(headers []string, rows [][]string, empty string) string {
	if len(rows) == 0 {
		return empty
	}
	b := strings.Builder{}
	b.WriteString("| ")
	b.WriteString(strings.Join(headers, " | "))
	b.WriteString(" |\n|")
	for range headers {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")
	for _, row := range rows {
		b.WriteString("| ")
		for i := range headers {
			if i > 0 {
				b.WriteString(" | ")
			}
			if i < len(row) {
				b.WriteString(cleanCell(row[i]))
			}
		}
		b.WriteString(" |\n")
	}
	return b.String()
}

func cleanCell(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.TrimSpace(s)
	return truncateText(s, maxCellBytes)
}

func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return fmt.Sprintf("%s...", s[:max])
}

func shortID(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

func humanBytes(value float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	unit := 0
	for value >= 1024 {
		if unit >= len(units)-1 {
			break
		}
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func boolText(value bool) string {
	if value {
		return "sim"
	}
	return "não"
}

func int64Text(value int64) string {
	return strconv.FormatInt(value, 10)
}

func addLine(lines *[]string, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	*lines = append(*lines, fmt.Sprintf("**%s:** %s", label, value))
}

func formatInfo(info system.Info) string {
	lines := []string{}
	addLine(&lines, "Docker", fmt.Sprintf("%s %s", info.Name, info.ServerVersion))
	addLine(&lines, "OS", fmt.Sprintf("%s (%s/%s)", info.OperatingSystem, info.OSType, info.Architecture))
	addLine(&lines, "Kernel", info.KernelVersion)
	if info.NCPU > 0 {
		addLine(&lines, "CPU", fmt.Sprintf("%d vCPU", info.NCPU))
	}
	if info.MemTotal > 0 {
		addLine(&lines, "RAM", humanBytes(float64(info.MemTotal)))
	}
	addLine(&lines, "Containers", fmt.Sprintf(
		"running %d / paused %d / stopped %d (total %d)",
		info.ContainersRunning,
		info.ContainersPaused,
		info.ContainersStopped,
		info.Containers,
	))
	addLine(&lines, "Images", strconv.Itoa(info.Images))
	storage := info.Driver
	if info.DockerRootDir != "" {
		storage = fmt.Sprintf("%s @ %s", info.Driver, info.DockerRootDir)
	}
	addLine(&lines, "Storage", storage)
	addLine(&lines, "Logging", info.LoggingDriver)
	cgroup := info.CgroupDriver
	if info.CgroupVersion != "" {
		cgroup = fmt.Sprintf("%s %s", info.CgroupDriver, info.CgroupVersion)
	}
	addLine(&lines, "Cgroup", cgroup)
	if len(lines) == 0 {
		return "🐳 Docker info\n\nDocker info indisponível."
	}
	return fmt.Sprintf("🐳 Docker info\n\n%s", strings.Join(lines, "\n"))
}

func formatContainers(items []container.Summary) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			shortID(item.ID),
			formatNames(item.Names),
			item.Image,
			string(item.State),
			item.Status,
			formatPorts(item.Ports),
		})
	}
	return fmt.Sprintf("🐳 Containers\n\n%s", table(
		[]string{"ID", "Name", "Image", "State", "Status", "Ports"},
		rows,
		"Nenhum container encontrado.",
	))
}

func formatNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, strings.TrimPrefix(name, "/"))
	}
	return strings.Join(out, ", ")
}

func formatPorts(ports []container.PortSummary) string {
	if len(ports) == 0 {
		return ""
	}
	out := make([]string, 0, len(ports))
	for _, port := range ports {
		host := ""
		if port.IP.IsValid() {
			host = port.IP.String()
		}
		if port.PublicPort > 0 {
			if host == "" {
				out = append(out, fmt.Sprintf("%d->%d/%s", port.PublicPort, port.PrivatePort, port.Type))
				continue
			}
			out = append(out, fmt.Sprintf("%s:%d->%d/%s", host, port.PublicPort, port.PrivatePort, port.Type))
			continue
		}
		out = append(out, fmt.Sprintf("%d/%s", port.PrivatePort, port.Type))
	}
	return strings.Join(out, ", ")
}

func formatImages(items []image.Summary) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		repo := strings.Join(item.RepoTags, ", ")
		if repo == "" {
			repo = "<none>"
		}
		created := "-"
		if item.Created > 0 {
			created = time.Unix(item.Created, 0).Format("2006-01-02 15:04")
		}
		containers := "-"
		if item.Containers >= 0 {
			containers = int64Text(item.Containers)
		}
		rows = append(rows, []string{
			repo,
			shortID(item.ID),
			created,
			humanBytes(float64(item.Size)),
			containers,
		})
	}
	return fmt.Sprintf("🖼️ Images\n\n%s", table(
		[]string{"Repository:Tag", "ID", "Created", "Size", "Containers"},
		rows,
		"Nenhuma imagem encontrada.",
	))
}

func formatStats(items []container.StatsResponse) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimPrefix(item.Name, "/")
		if name == "" {
			name = shortID(item.ID)
		}
		rows = append(rows, []string{
			name,
			cpuPercent(item),
			memUsage(item),
			memPercent(item),
			netIO(item),
			blockIO(item),
			strconv.FormatUint(item.PidsStats.Current, 10),
		})
	}
	return fmt.Sprintf("📊 Docker stats\n\n%s", table(
		[]string{"Name", "CPU %", "Mem usage", "Mem %", "Net I/O (rx/tx)", "Block I/O (r/w)", "PIDs"},
		rows,
		"Nenhum container em execução.",
	))
}

func cpuPercent(item container.StatsResponse) string {
	cpuDelta := float64(item.CPUStats.CPUUsage.TotalUsage) - float64(item.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(item.CPUStats.SystemUsage) - float64(item.PreCPUStats.SystemUsage)
	cpus := float64(item.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(item.CPUStats.CPUUsage.PercpuUsage))
	}
	if cpus == 0 {
		return "-"
	}
	if systemDelta <= 0 {
		return "-"
	}
	if cpuDelta < 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", (cpuDelta/systemDelta)*cpus*100)
}

func memUsage(item container.StatsResponse) string {
	usage := item.MemoryStats.Usage
	cache := item.MemoryStats.Stats["inactive_file"]
	if usage > cache {
		usage -= cache
	}
	return fmt.Sprintf("%s / %s", humanBytes(float64(usage)), humanBytes(float64(item.MemoryStats.Limit)))
}

func memPercent(item container.StatsResponse) string {
	usage := item.MemoryStats.Usage
	cache := item.MemoryStats.Stats["inactive_file"]
	if usage > cache {
		usage -= cache
	}
	if item.MemoryStats.Limit == 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", float64(usage)/float64(item.MemoryStats.Limit)*100)
}

func netIO(item container.StatsResponse) string {
	var rx uint64
	var tx uint64
	for _, networkStats := range item.Networks {
		rx += networkStats.RxBytes
		tx += networkStats.TxBytes
	}
	return fmt.Sprintf("%s / %s", humanBytes(float64(rx)), humanBytes(float64(tx)))
}

func blockIO(item container.StatsResponse) string {
	var read uint64
	var write uint64
	for _, entry := range item.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			read += entry.Value
		case "write":
			write += entry.Value
		}
	}
	return fmt.Sprintf("%s / %s", humanBytes(float64(read)), humanBytes(float64(write)))
}

func formatVolumes(items []volume.Volume) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Name,
			item.Driver,
			item.Scope,
			item.Mountpoint,
		})
	}
	return fmt.Sprintf("💾 Volumes\n\n%s", table(
		[]string{"Name", "Driver", "Scope", "Mountpoint"},
		rows,
		"Nenhum volume encontrado.",
	))
}

func formatNetworks(items []network.Summary) string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.Name,
			shortID(item.ID),
			item.Driver,
			item.Scope,
			boolText(item.EnableIPv6),
			boolText(item.Internal),
		})
	}
	return fmt.Sprintf("🌐 Networks\n\n%s", table(
		[]string{"Name", "ID", "Driver", "Scope", "IPv6", "Internal"},
		rows,
		"Nenhuma network encontrada.",
	))
}

func formatLogs(target string, tail int, text string) string {
	if strings.TrimSpace(text) == "" {
		return fmt.Sprintf("📜 `%s` — sem logs.", target)
	}
	truncated := len(text) > maxLogBytes
	if truncated {
		text = truncateText(text, maxLogBytes)
	}
	header := fmt.Sprintf("📜 Logs de `%s` (tail %d)", target, tail)
	if truncated {
		header = fmt.Sprintf("%s (truncado)", header)
	}
	return fmt.Sprintf("%s\n\n```text\n%s\n```", header, text)
}

func redactEnv(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if strings.EqualFold(key, "env") {
				continue
			}
			out[key] = redactEnv(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactEnv(item)
		}
		return out
	default:
		return value
	}
}

func formatInspect(target string, value any) string {
	if value == nil {
		return fmt.Sprintf("🔍 `%s` — sem dados.", target)
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("🔍 `%s` — falha ao formatar JSON: %v", target, err)
	}
	text := string(pretty)
	truncated := len(text) > maxInspectBytes
	if truncated {
		text = truncateText(text, maxInspectBytes)
	}
	header := fmt.Sprintf("🔍 `%s`", target)
	if truncated {
		header = fmt.Sprintf("%s (truncado)", header)
	}
	return fmt.Sprintf("%s\n\n```json\n%s\n```", header, text)
}

func formatDiskUsage(usage mobyclient.DiskUsageResult) string {
	rows := [][]string{
		{
			"Containers",
			int64Text(usage.Containers.TotalCount),
			int64Text(usage.Containers.ActiveCount),
			humanBytes(float64(usage.Containers.TotalSize)),
			humanBytes(float64(usage.Containers.Reclaimable)),
		},
		{
			"Images",
			int64Text(usage.Images.TotalCount),
			int64Text(usage.Images.ActiveCount),
			humanBytes(float64(usage.Images.TotalSize)),
			humanBytes(float64(usage.Images.Reclaimable)),
		},
		{
			"Volumes",
			int64Text(usage.Volumes.TotalCount),
			int64Text(usage.Volumes.ActiveCount),
			humanBytes(float64(usage.Volumes.TotalSize)),
			humanBytes(float64(usage.Volumes.Reclaimable)),
		},
		{
			"Build cache",
			int64Text(usage.BuildCache.TotalCount),
			int64Text(usage.BuildCache.ActiveCount),
			humanBytes(float64(usage.BuildCache.TotalSize)),
			humanBytes(float64(usage.BuildCache.Reclaimable)),
		},
	}
	return fmt.Sprintf("💾 Docker disk usage\n\n%s", table(
		[]string{"Type", "Total", "Active", "Size", "Reclaimable"},
		rows,
		"Sem dados.",
	))
}

func formatRun(name, imageName string, result mobyclient.ContainerCreateResult, pulled bool) string {
	header := "🚀 Container criado"
	if name != "" {
		header = fmt.Sprintf("%s: `%s`", header, name)
	}
	b := strings.Builder{}
	fmt.Fprintf(&b, "%s\n\n- Imagem: `%s`\n- ID: `%s`\n- Pull: %s", header, imageName, result.ID, boolText(pulled))
	if len(result.Warnings) > 0 {
		b.WriteString("\n\n**Warnings:**\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(&b, "- %s\n", warning)
		}
	}
	return b.String()
}

func formatRemove(name string) string {
	return fmt.Sprintf("🗑️ Container removido: `%s`.", name)
}
