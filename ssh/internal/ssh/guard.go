package ssh

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var blockedPaths = []string{
	"/etc",
	"/boot",
	"/usr",
	"/bin",
	"/sbin",
	"/lib",
	"/lib64",
	"/sys",
	"/proc",
	"/dev",
	"/root",
}

type rule struct {
	name string
	re   *regexp.Regexp
}

var dangerousRules = []rule{
	{name: "mkfs", re: regexp.MustCompile(`(?i)\bmkfs(\.[a-z0-9]+)?\b`)},
	{name: "wipefs", re: regexp.MustCompile(`(?i)\bwipefs\b`)},
	{name: "dd em device", re: regexp.MustCompile(`(?i)\bdd\b[^\n]*\bof\s*=\s*/dev/`)},
	{name: "desligar ou reiniciar", re: regexp.MustCompile(`(?i)\b(shutdown|reboot|halt|poweroff)\b`)},
	{name: "systemctl de energia", re: regexp.MustCompile(`(?i)\bsystemctl\s+(poweroff|reboot|halt|kexec|rescue|emergency)\b`)},
	{name: "init 0/6", re: regexp.MustCompile(`(?i)\binit\s+[06]\b`)},
	{name: "rm -rf na raiz", re: regexp.MustCompile(`(?i)\brm\s+(-[a-z]*[rf][a-z]*\s+)+(/(\s|$|\*)|~/?(\s|$))`)},
	{name: "escrita em device", re: regexp.MustCompile(`>\s*/dev/(sd|nvme|hd|vd|disk)`)},
	{name: "fork bomb", re: regexp.MustCompile(`:\s*\(\s*\)\s*\{`)},
	{name: "rm recursivo em dirs críticos", re: regexp.MustCompile(`(?i)\brm\s+(-[a-z]*\s+)*(--no-preserve-root|/(etc|bin|sbin|usr|lib|lib64|boot|var|home|root|dev|proc|sys)\b)`)},
	{name: "rm --no-preserve-root", re: regexp.MustCompile(`(?i)--no-preserve-root`)},
	{name: "shred em device", re: regexp.MustCompile(`(?i)\bshred\b[^\n]*/dev/`)},
	{name: "escrita de arquivo em device", re: regexp.MustCompile(`(?i)\b(cp|cat|tee|pv)\b[^\n]*(of=|>|\s)/dev/(sd|nvme|hd|vd|mmcblk|disk)`)},
	{name: "dd de /dev/zero ou random", re: regexp.MustCompile(`(?i)\bdd\b[^\n]*\bif\s*=\s*/dev/(zero|u?random)`)},
	{name: "particionamento", re: regexp.MustCompile(`(?i)\b(fdisk|sfdisk|gdisk|cfdisk|parted|sgdisk)\b`)},
	{name: "cryptsetup destrutivo", re: regexp.MustCompile(`(?i)\bcryptsetup\s+(luksFormat|erase|luksErase|reencrypt)\b`)},
	{name: "lvm/mdadm destrutivo", re: regexp.MustCompile(`(?i)\b(lvremove|vgremove|pvremove|mdadm\s+--(zero-superblock|stop))\b`)},
	{name: "zpool/zfs destroy", re: regexp.MustCompile(`(?i)\b(zpool|zfs)\s+destroy\b`)},
	{name: "mkswap/btrfs device", re: regexp.MustCompile(`(?i)\b(mkswap|btrfs\s+device\s+(delete|remove))\b`)},
	{name: "chmod/chown recursivo na raiz", re: regexp.MustCompile(`(?i)\b(chmod|chown|chgrp)\s+(-[a-z]*R[a-z]*|--recursive)\s+[^\n]*\s/(\s|$)`)},
	{name: "chmod 000/777 em sistema", re: regexp.MustCompile(`(?i)\bchmod\s+(-R\s+)?(000|777)\s+/`)},
	{name: "mv para /dev/null", re: regexp.MustCompile(`(?i)\bmv\s+[^\n]*\s+/dev/null\b`)},
	{name: "truncar arquivo crítico", re: regexp.MustCompile(`>\s*/etc/(passwd|shadow|fstab|sudoers|group)\b`)},
	{name: "editar credenciais", re: regexp.MustCompile(`(?i)\b(visudo|vipw|passwd\s+root|userdel\s+-r?\s*root)\b`)},
	{name: "matar todos os processos", re: regexp.MustCompile(`(?i)\b(kill|killall|pkill)\s+(-9\s+|-KILL\s+)?(-1|1)\b|\bkillall5\b`)},
	{name: "sysrq trigger", re: regexp.MustCompile(`/proc/sysrq-trigger`)},
	{name: "escrita em /proc ou /sys", re: regexp.MustCompile(`>\s*/(proc|sys)/`)},
	{name: "pipe de download para shell", re: regexp.MustCompile(`(?i)\b(curl|wget|fetch)\b[^\n|]*\|\s*(sudo\s+)?(ba|z|da|k)?sh\b`)},
	{name: "base64 para shell", re: regexp.MustCompile(`(?i)base64\s+(-d|--decode)[^\n]*\|\s*(sudo\s+)?(ba|z)?sh\b`)},
	{name: "eval de download", re: regexp.MustCompile(`(?i)\beval\s+["']?\$\((curl|wget)`)},
	{name: "flush de firewall", re: regexp.MustCompile(`(?i)\b(iptables|ip6tables|nft)\s+(-F|--flush|flush\s+ruleset)\b|\bufw\s+(disable|reset)\b`)},
	{name: "derrubar rede", re: regexp.MustCompile(`(?i)\b(ip\s+link\s+set\s+\S+\s+down|ifdown|nmcli\s+networking\s+off)\b`)},
	{name: "parar sshd ou rede", re: regexp.MustCompile(`(?i)\bsystemctl\s+(stop|disable|mask)\s+(sshd?|networking|NetworkManager)\b`)},
	{name: "desmontar a raiz", re: regexp.MustCompile(`(?i)\b(umount\s+(-[a-z]+\s+)*/(\s|$)|mount\s+[^\n]*remount,ro\s+/(\s|$))`)},
	{name: "remover pacote essencial", re: regexp.MustCompile(`(?i)\b(apt(-get)?\s+(purge|remove)\s+[^\n]*(libc6|systemd|bash|coreutils)|dpkg\s+--purge\s+--force|yum\s+remove\s+glibc|rpm\s+-e\s+--nodeps)\b`)},
	{name: "apagar histórico ou logs", re: regexp.MustCompile(`(?i)(\bhistory\s+-c\b|>\s*/var/log/|\brm\s+[^\n]*/var/log\b|\bjournalctl\s+--vacuum)`)},
	{name: "crontab -r", re: regexp.MustCompile(`(?i)\bcrontab\s+-r\b`)},
	{name: "docker/k8s destrutivo", re: regexp.MustCompile(`(?i)\b(docker\s+system\s+prune\s+[^\n]*-a|docker\s+rm\s+-f\s+\$\(|kubectl\s+delete\s+(ns|namespace|--all)\b)`)},
	{name: "find -delete na raiz", re: regexp.MustCompile(`(?i)\bfind\s+/\s+[^\n]*(-delete|-exec\s+rm)\b`)},
}

func checkCommand(cfg Config, command string) error {
	err := checkDangerous(command)
	if err != nil {
		return err
	}
	return checkAllow(cfg.Alias, cfg.allow, command)
}

func checkDangerous(command string) error {
	for _, r := range dangerousRules {
		if r.re.MatchString(command) {
			return fmt.Errorf("comando bloqueado (%s)", r.name)
		}
	}
	return nil
}

func checkAllow(alias string, allow []string, command string) error {
	if len(allow) == 0 {
		return nil
	}
	for _, segment := range commandSegments(command) {
		word := firstWord(segment)
		if word == "" {
			continue
		}
		if !slices.Contains(allow, word) {
			return fmt.Errorf("comando %q não permitido: defina %s", word, envName(alias, "ALLOW"))
		}
	}
	return nil
}

func checkProtected(path string) error {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if strings.HasPrefix(cleaned, "~") {
		cleaned = strings.TrimPrefix(cleaned, "~")
	}
	for _, prefix := range blockedPaths {
		if cleaned == prefix {
			return fmt.Errorf("operação recusada em %s: caminho protegido", prefix)
		}
		if strings.HasPrefix(cleaned, prefix+"/") {
			return fmt.Errorf("operação recusada em %s: caminho protegido", prefix)
		}
	}
	for _, segment := range strings.Split(cleaned, "/") {
		if segment == ".ssh" {
			return errors.New("operação recusada em ~/.ssh: caminho protegido")
		}
	}
	return nil
}

func commandSegments(command string) []string {
	replacer := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n", "&", "\n")
	return strings.Split(replacer.Replace(command), "\n")
}

func firstWord(segment string) string {
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}
