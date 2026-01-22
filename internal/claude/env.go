package claude

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

const (
	// DefaultBridgeInterface is the default LXC bridge interface
	DefaultBridgeInterface = "lxdbr0"
)

// BuildEnvironment creates an Environment from the given parameters.
func BuildEnvironment(host string, port int, project, agent, projectPath, bareRepoPath string) *Environment {
	return &Environment{
		AiryraHost:    host,
		AiryraPort:    port,
		AiryraProject: project,
		AiryraAgent:   agent,
		ProjectPath:   projectPath,
		BareRepoPath:  bareRepoPath,
	}
}

// ToEnvVars converts the Environment to a slice of KEY=VALUE strings.
func (e *Environment) ToEnvVars() []string {
	return []string{
		fmt.Sprintf("AIRYRA_HOST=%s", e.AiryraHost),
		fmt.Sprintf("AIRYRA_PORT=%d", e.AiryraPort),
		fmt.Sprintf("AIRYRA_PROJECT=%s", e.AiryraProject),
		fmt.Sprintf("AIRYRA_AGENT=%s", e.AiryraAgent),
		fmt.Sprintf("ISOLLM_PROJECT_PATH=%s", e.ProjectPath),
		fmt.Sprintf("ISOLLM_BARE_REPO=%s", e.BareRepoPath),
	}
}

// ToEnvFile converts the Environment to a shell-sourceable file content.
func (e *Environment) ToEnvFile() string {
	var b strings.Builder
	b.WriteString("# isollm environment variables\n")
	b.WriteString("# Source this file to set up the Claude environment\n\n")
	b.WriteString(fmt.Sprintf("export AIRYRA_HOST=%q\n", e.AiryraHost))
	b.WriteString(fmt.Sprintf("export AIRYRA_PORT=%d\n", e.AiryraPort))
	b.WriteString(fmt.Sprintf("export AIRYRA_PROJECT=%q\n", e.AiryraProject))
	b.WriteString(fmt.Sprintf("export AIRYRA_AGENT=%q\n", e.AiryraAgent))
	b.WriteString(fmt.Sprintf("export ISOLLM_PROJECT_PATH=%q\n", e.ProjectPath))
	b.WriteString(fmt.Sprintf("export ISOLLM_BARE_REPO=%q\n", e.BareRepoPath))
	return b.String()
}

// GetHostIP returns the IP address of the host that is accessible from LXC containers.
// It tries to get the IP from the lxdbr0 bridge interface first, then falls back
// to other methods.
func GetHostIP() (string, error) {
	// Try the LXC bridge interface first
	ip, err := getInterfaceIP(DefaultBridgeInterface)
	if err == nil && ip != "" {
		return ip, nil
	}

	// Fall back to using ip route to find the default gateway interface
	ip, err = getDefaultRouteIP()
	if err == nil && ip != "" {
		return ip, nil
	}

	return "", fmt.Errorf("could not determine host IP for LXC containers")
}

// getInterfaceIP returns the IPv4 address of a network interface.
func getInterfaceIP(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipv4 := ipnet.IP.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no IPv4 address found for interface %s", ifaceName)
}

// getDefaultRouteIP returns the IP address of the interface with the default route.
func getDefaultRouteIP() (string, error) {
	cmd := exec.Command("ip", "route", "get", "1.1.1.1")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse output like: "1.1.1.1 via 192.168.1.1 dev eth0 src 192.168.1.100"
	fields := strings.Fields(string(output))
	for i, field := range fields {
		if field == "src" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}

	return "", fmt.Errorf("could not parse default route output")
}
