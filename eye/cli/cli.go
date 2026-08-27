package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var version = "dev"

const (
	installBinary = "/opt/panoptes/eye"
	installState  = "/var/lib/panoptes/eye"
	installEnv    = "/etc/panoptes/eye.env"
	installUnit   = "/etc/systemd/system/panoptes-eye.service"
	installDropIn = "/etc/systemd/system/panoptes-eye.service.d"
	installLink   = "/usr/local/bin/eye"
	installUser   = "panoptes-eye"
	serviceName   = "panoptes-eye.service"
	releaseBase   = "https://github.com/Kirragami/panoptes/releases/latest/download"

	eyeIDFile   = "eye-id"
	epithetFile = "epithet"
	brandFile   = "brand"
	sealFile    = "seal"
)

func Handle(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "version":
		fmt.Println(version)
		return true, nil
	case "status":
		return true, printStatus()
	case "update":
		return true, update()
	case "uninstall":
		return true, uninstall(args[1:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return true, nil
	default:
		return true, fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

const usage = `usage:
	eye         -  run the Eye daemon
	eye version
	eye status
	eye update
	eye uninstall [--yes]
`

func printStatus() error {
	if err := loadInstallEnv(installEnv); err != nil {
		return err
	}

	stateDir := strings.TrimSpace(os.Getenv("PANOPTES_STATE_DIR"))
	if stateDir == "" {
		if _, err := os.Stat(installState); err == nil {
			stateDir = installState
		} else {
			stateDir = "./state"
		}
	}

	endpoint := strings.TrimSpace(os.Getenv("PANOPTICON_ENDPOINT"))
	eyeID, _ := os.ReadFile(filepath.Join(stateDir, eyeIDFile))
	epithet, _ := os.ReadFile(filepath.Join(stateDir, epithetFile))
	_, brandErr := os.Stat(filepath.Join(stateDir, brandFile))
	_, sealErr := os.Stat(filepath.Join(stateDir, sealFile))

	branded := "no"
	if brandErr == nil {
		branded = "yes"
	}
	sealPresent := "no"
	if sealErr == nil {
		sealPresent = "yes"
	}

	fmt.Printf("version		 %s\n", version)
	fmt.Printf("state		 %s\n", stateDir)
	fmt.Printf("endpoint	 %s\n", endpoint)
	fmt.Printf("eye ID		 %s\n", strings.TrimSpace(string(eyeID)))
	fmt.Printf("epithet		 %s\n", strings.TrimSpace(string(epithet)))
	fmt.Printf("branded		 %s\n", branded)
	fmt.Printf("seal present %s\n", sealPresent)
	fmt.Printf("service		 %s\n", serviceState(serviceName))
	return nil
}

func update() error {
	if err := requireRoot(); err != nil {
		return err
	}
	if runtime.GOOS != "linux" {
		return fmt.Errorf("update is only published for linux")
	}
	if _, err := os.Stat(installBinary); err != nil {
		return fmt.Errorf("Eye is not installed at %s", installBinary)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	remoteVersion, err := fetchReleaseText(client, releaseBase+"/VERSION")
	if err != nil {
		return fmt.Errorf("recall remote Eye version: %w", err)
	}
	if remoteVersion == version {
		fmt.Printf("Eye is already %s\n", version)
		return nil
	}

	asset := "eye-linux-" + runtime.GOARCH
	checksums, err := fetchReleaseText(client, releaseBase+"/checksums.txt")
	if err != nil {
		return fmt.Errorf("recall Eye checksums: %w", err)
	}
	wantHash, err := checksumForAsset(checksums, asset)
	if err != nil {
		return err
	}

	payload, err := fetchReleaseBytes(client, releaseBase+"/"+asset)
	if err != nil {
		return fmt.Errorf("download Eye: %w", err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != wantHash {
		return fmt.Errorf("downloaded Eye checksum does not match")
	}

	if err := replaceExecutable(installBinary, payload); err != nil {
		return err
	}
	_ = runRootCommand("systemctl", "restart", serviceName)
	fmt.Printf("Eye updated from %s to %s\n", version, remoteVersion)
	return nil
}

func uninstall(args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	if _, err := os.Stat(installBinary); err != nil && os.IsNotExist(err) {
		if _, unitErr := os.Stat(installUnit); unitErr != nil && os.IsNotExist(unitErr) {
			return fmt.Errorf("Eye is not installed")
		}
	}

	if err := confirmDestructive(
		args,
		"This removes the Eye unit, binary, state, and user panoptes-eye.\nType yes to continue: ",
	); err != nil {
		return err
	}

	_ = runRootCommand("systemctl", "stop", serviceName)
	_ = runRootCommand("systemctl", "disable", serviceName)
	if err := removePath(installUnit); err != nil {
		return err
	}
	if err := removePath(installDropIn); err != nil {
		return err
	}
	if err := removePath(installBinary); err != nil {
		return err
	}
	if err := removePath(installEnv); err != nil {
		return err
	}
	if err := removePath(installState); err != nil {
		return err
	}
	if target, err := os.Readlink(installLink); err == nil && target == installBinary {
		_ = os.Remove(installLink)
	}
	if err := deleteUnixUser(installUser); err != nil {
		return err
	}
	_ = runRootCommand("systemctl", "daemon-reload")
	removeEmptyDir("/opt/panoptes")
	removeEmptyDir("/etc/panoptes")
	removeEmptyDir("/var/lib/panoptes")
	fmt.Println("Eye removed")
	return nil
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}
	return nil
}

func confirmDestructive(args []string, prompt string) error {
	for _, argument := range args {
		if argument == "--yes" || argument == "-y" {
			return nil
		}
	}

	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("refusing without --yes")
	}

	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(line)) != "yes" && strings.ToLower(strings.TrimSpace(line)) != "y" {
		return fmt.Errorf("cancelled")
	}
	return nil
}

func runRootCommand(name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func loadInstallEnv(path string) error {
	values, err := parseEnvFile(path)
	if err != nil {
		return err
	}
	for name, value := range values {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			_ = os.Setenv(name, value)
		}
	}
	return nil
}

func parseEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(name) == "" {
			continue
		}
		value = strings.ReplaceAll(value, "$$", "$")
		value = strings.ReplaceAll(value, "%%", "%")
		values[strings.TrimSpace(name)] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func serviceState(unit string) string {
	output, err := exec.Command("systemctl", "is-active", unit).Output()
	state := strings.TrimSpace(string(output))
	if state == "" {
		if err != nil {
			return "unknown"
		}
		return "inactive"
	}
	return state
}

func fetchReleaseText(client *http.Client, url string) (string, error) {
	body, err := fetchReleaseBytes(client, url)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func fetchReleaseBytes(client *http.Client, url string) ([]byte, error) {
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, response.Status)
	}

	const maxAsset = 200 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAsset+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxAsset {
		return nil, fmt.Errorf("%s is larger than %d bytes", url, maxAsset)
	}
	return body, nil
}

func checksumForAsset(checksums, asset string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(checksums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if filepath.Base(fields[len(fields)-1]) == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksum for %s was not published", asset)
}

func replaceExecutable(path string, payload []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	tempFile, err := os.CreateTemp(directory, ".eye-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.Write(payload); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Chmod(0755); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return os.Rename(tempPath, path)
}

func removePath(path string) error {
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func removeEmptyDir(path string) {
	_ = os.Remove(path)
}

func deleteUnixUser(name string) error {
	if _, err := user.Lookup(name); err != nil {
		return nil
	}
	if err := runRootCommand("userdel", name); err != nil {
		return err
	}
	if _, err := user.LookupGroup(name); err == nil {
		_ = runRootCommand("groupdel", name)
	}
	return nil
}
