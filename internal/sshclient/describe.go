package sshclient

import (
	"fmt"
	"strings"

	"github.com/muhammadjihad/spock/internal/config"
)

type NodeInfo struct {
	Hostname  string
	OS        string
	Kernel    string
	CPUModel  string
	CPUCores  string
	MemInfo   string
	DiskInfo  string
	DiskTotal string
	GPU       string
	HasSudo   bool
	Sysctl    string
	Systemd   string
}

// DescribeNode connects to a remote host and gathers system information.
func DescribeNode(opts ConnectOptions, cfg *config.AppConfig) (*NodeInfo, error) {
	script := `
echo "===HOSTNAME==="
hostname 2>/dev/null || echo "Unknown"
echo "===OS==="
(. /etc/os-release 2>/dev/null && echo "$NAME $VERSION") || echo "Unknown"
echo "===KERNEL==="
uname -r 2>/dev/null || echo "Unknown"
echo "===CPU_MODEL==="
grep "model name" /proc/cpuinfo 2>/dev/null | head -1 | cut -d: -f2 | xargs || echo "Unknown"
echo "===CPU_CORES==="
nproc 2>/dev/null || echo "Unknown"
echo "===MEMORY==="
free -h 2>/dev/null | grep -E "Mem:|Swap:" || echo "Unavailable"
echo "===DISK==="
df -h 2>/dev/null | grep -E "^/|^Filesystem" || echo "Unavailable"
echo "===DISK_TOTAL==="
df -h --total 2>/dev/null | awk '/^total/{print $2}' || df -h 2>/dev/null | awk '/^\//{s+=$2}END{printf "%.0fG\n",s}' || echo "Unknown"
echo "===GPU==="
gpu_found=0

# Method 1: lspci (catch VGA, 3D, display, AND virtio-gpu)
if command -v lspci >/dev/null 2>&1; then
  gpu_lines=$(lspci 2>/dev/null | grep -iE '(vga|3d controller|display|virtio.*gpu)')
  if [ -n "$gpu_lines" ]; then
    echo "$gpu_lines" | sed 's/^[^ ]* //'
    gpu_found=1
  fi
fi

# Method 2: vulkaninfo (critical for VMs with virtio-gpu, e.g. VirtualBox on Apple Silicon)
# Filter out llvmpipe (software renderer) — only show real GPUs
if [ "$gpu_found" -eq 0 ] && command -v vulkaninfo >/dev/null 2>&1; then
  vk=$(vulkaninfo --summary 2>/dev/null | grep -E "deviceName|deviceType" | paste - - 2>/dev/null | grep -iv "llvmpipe" | grep -iv "DEVICE_TYPE_CPU" | sed 's/.*deviceName.*=[ ]*//' | sed 's/[ ]*deviceType.*//')
  if [ -z "$vk" ]; then
    # If all filtered out, show all non-CPU devices including names with type
    vk=$(vulkaninfo --summary 2>/dev/null | grep "deviceName" | sed 's/.*=[ ]*//' | grep -iv "llvmpipe")
  fi
  if [ -z "$vk" ]; then
    # Last try: just get all device names
    vk=$(vulkaninfo --summary 2>/dev/null | grep "deviceName" | sed 's/.*=[ ]*//')
  fi
  if [ -n "$vk" ]; then
    echo "$vk"
    gpu_found=1
  fi
fi

# Method 3: nvidia-smi
if [ "$gpu_found" -eq 0 ] && command -v nvidia-smi >/dev/null 2>&1; then
  nv=$(nvidia-smi --query-gpu=name,memory.total --format=csv,noheader 2>/dev/null)
  if [ -n "$nv" ]; then
    echo "NVIDIA: $nv"
    gpu_found=1
  fi
fi

# Method 4: sysfs PCI class scan
if [ "$gpu_found" -eq 0 ]; then
  for dev in /sys/bus/pci/devices/*; do
    class=$(cat "$dev/class" 2>/dev/null)
    case "$class" in
      0x0300*|0x0302*|0x0380*)
        vendor=$(cat "$dev/vendor" 2>/dev/null)
        device=$(cat "$dev/device" 2>/dev/null)
        case "$vendor" in
          0x10de) vname="NVIDIA" ;; 0x1002) vname="AMD" ;; 0x8086) vname="Intel" ;; *) vname="vendor=$vendor" ;;
        esac
        echo "$vname device=$device"
        gpu_found=1
        ;;
    esac
  done
fi

# Method 5: /dev/dri render nodes
if [ "$gpu_found" -eq 0 ] && [ -d /dev/dri ]; then
  renders=$(ls /dev/dri/renderD* 2>/dev/null | wc -l)
  if [ "$renders" -gt 0 ]; then
    echo "GPU detected ($renders render node(s) in /dev/dri)"
    gpu_found=1
  fi
fi

[ "$gpu_found" -eq 0 ] && echo "None"
echo "===SUDO==="
sudo -n true 2>/dev/null && echo "YES" || echo "NO"
echo "===SYSCTL==="
sysctl vm.swappiness fs.file-max net.ipv4.ip_forward net.ipv4.tcp_max_syn_backlog net.core.somaxconn net.core.netdev_max_backlog kernel.pid_max net.ipv4.tcp_fin_timeout net.ipv4.tcp_keepalive_time net.ipv4.tcp_tw_reuse 2>/dev/null || echo "Unavailable"
echo "===SYSTEMD==="
systemctl list-units --type=service --state=running --no-pager --no-legend 2>/dev/null | awk '{print $1}' | head -40 || echo "Unavailable"
echo "===END==="
`
	output, err := RunCommand(opts, script, cfg)
	if err != nil {
		if output == "" {
			return nil, fmt.Errorf("failed to describe node: %w", err)
		}
	}

	return parseNodeInfo(output), nil
}

func parseNodeInfo(output string) *NodeInfo {
	info := &NodeInfo{}
	sections := parseSections(output)

	info.Hostname = strings.TrimSpace(sections["HOSTNAME"])
	info.OS = strings.TrimSpace(sections["OS"])
	info.Kernel = strings.TrimSpace(sections["KERNEL"])
	info.CPUModel = strings.TrimSpace(sections["CPU_MODEL"])
	info.CPUCores = strings.TrimSpace(sections["CPU_CORES"])
	info.MemInfo = strings.TrimSpace(sections["MEMORY"])
	info.DiskInfo = strings.TrimSpace(sections["DISK"])
	info.DiskTotal = strings.TrimSpace(sections["DISK_TOTAL"])
	info.GPU = strings.TrimSpace(sections["GPU"])
	info.HasSudo = strings.TrimSpace(sections["SUDO"]) == "YES"
	info.Sysctl = strings.TrimSpace(sections["SYSCTL"])
	info.Systemd = strings.TrimSpace(sections["SYSTEMD"])

	return info
}

func parseSections(output string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(output, "\n")

	currentKey := ""
	var currentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "===") && strings.HasSuffix(line, "===") {
			if currentKey != "" {
				sections[currentKey] = strings.Join(currentLines, "\n")
			}
			currentKey = strings.Trim(line, "=")
			currentLines = nil
		} else if currentKey != "" {
			currentLines = append(currentLines, line)
		}
	}
	if currentKey != "" {
		sections[currentKey] = strings.Join(currentLines, "\n")
	}
	return sections
}

// FormatNodeInfo returns a nicely formatted string of the node information.
func FormatNodeInfo(info *NodeInfo, connName string) string {
	var b strings.Builder

	header := fmt.Sprintf(" System Information: %s ", connName)
	width := 70
	if len(header) > width-4 {
		width = len(header) + 4
	}

	bar := strings.Repeat("─", width-2)
	b.WriteString(fmt.Sprintf("╭%s╮\n", bar))
	b.WriteString(fmt.Sprintf("│%-*s│\n", width-2, header))
	b.WriteString(fmt.Sprintf("├%s┤\n", bar))

	// System
	writeField(&b, width, "Hostname:", info.Hostname)
	writeField(&b, width, "OS:", info.OS)
	writeField(&b, width, "Kernel:", info.Kernel)
	b.WriteString(fmt.Sprintf("│%-*s│\n", width-2, ""))

	// CPU
	writeField(&b, width, "CPU:", info.CPUModel)
	writeField(&b, width, "Cores:", info.CPUCores)
	b.WriteString(fmt.Sprintf("│%-*s│\n", width-2, ""))

	// GPU
	gpuLines := strings.Split(info.GPU, "\n")
	writeField(&b, width, "GPU:", strings.TrimSpace(gpuLines[0]))
	for _, gl := range gpuLines[1:] {
		gl = strings.TrimSpace(gl)
		if gl != "" {
			writeField(&b, width, "", gl)
		}
	}
	b.WriteString(fmt.Sprintf("│%-*s│\n", width-2, ""))

	// Sudo
	sudo := "Not available (user lacks sudo)"
	if info.HasSudo {
		sudo = "Available"
	}
	writeField(&b, width, "Sudo:", sudo)
	b.WriteString(fmt.Sprintf("│%-*s│\n", width-2, ""))

	// Memory
	writeSection(&b, width, bar, "Memory", info.MemInfo)

	// Disk
	diskHeader := "Disk Usage"
	if info.DiskTotal != "" && info.DiskTotal != "Unknown" {
		diskHeader = fmt.Sprintf("Disk Usage (total: %s)", info.DiskTotal)
	}
	writeSection(&b, width, bar, diskHeader, info.DiskInfo)

	// Sysctl
	b.WriteString(fmt.Sprintf("├%s┤\n", bar))
	b.WriteString(fmt.Sprintf("│  %-*s│\n", width-4, "Sysctl Configuration"))
	if !info.HasSudo && info.Sysctl == "Unavailable" {
		b.WriteString(fmt.Sprintf("│    %-*s│\n", width-6, "Requires sudo access"))
	} else {
		writeLines(&b, width, info.Sysctl)
	}

	// Systemd
	b.WriteString(fmt.Sprintf("├%s┤\n", bar))
	b.WriteString(fmt.Sprintf("│  %-*s│\n", width-4, "Running Services (systemd)"))
	if info.Systemd == "Unavailable" {
		b.WriteString(fmt.Sprintf("│    %-*s│\n", width-6, "systemd not available"))
	} else {
		writeLines(&b, width, info.Systemd)
	}

	b.WriteString(fmt.Sprintf("╰%s╯\n", bar))
	return b.String()
}

func writeField(b *strings.Builder, width int, label, value string) {
	if label != "" {
		text := fmt.Sprintf("  %-14s%s", label, value)
		if len(text) > width-2 {
			text = text[:width-3] + "…"
		}
		b.WriteString(fmt.Sprintf("│%-*s│\n", width-2, text))
	} else {
		text := fmt.Sprintf("                %s", value)
		if len(text) > width-2 {
			text = text[:width-3] + "…"
		}
		b.WriteString(fmt.Sprintf("│%-*s│\n", width-2, text))
	}
}

func writeSection(b *strings.Builder, width int, bar, title, content string) {
	b.WriteString(fmt.Sprintf("├%s┤\n", bar))
	b.WriteString(fmt.Sprintf("│  %-*s│\n", width-4, title))
	writeLines(b, width, content)
}

func writeLines(b *strings.Builder, width int, content string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > width-6 {
				line = line[:width-7] + "…"
			}
			b.WriteString(fmt.Sprintf("│    %-*s│\n", width-6, line))
		}
	}
}
