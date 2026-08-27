package cli

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

	"golang.org/x/crypto/argon2"
)

var version = "dev"

const (
	installBinary = "/opt/panoptes/panopticon"
	installState  = "/var/lib/panoptes/panopticon"
	installEnv    = "/etc/panoptes/panopticon.env"
	installUnit   = "/etc/systemd/system/panopticon.service"
	installLink   = "/usr/local/bin/panopticon"
	installUser   = "panopticon"
	serviceName   = "panopticon.service"
	releaseBase   = "https://github.com/Kirragami/panoptes/releases/latest/download"
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
	case "hash-password":
		return true, hashPanelPassword()
	case "help", "-h", "--help":
		fmt.Print(usage)
		return true, nil
	default:
		return true, fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

const usage = `usage:
	panopticon                      run the Panopticon daemon
	panopticon version              print the stamped version
	panopticon status               print install and state
	panopticon update               replace the installed binary
	panopticon uninstall [--yes]    remove the unit, Chronicle, and user
	panopticon hash-password        read a password from stdin, print an Argon2id hash
`

func printStatus() error {
	if err := loadInstallEnv(installEnv); err != nil {
		return err
	}

	chronicle := strings.TrimSpace(os.Getenv("PANOPTICON_CHRONICLE"))
	if chronicle == "" {
		chronicle = "./panopticon.chronicle.db"
	}
	panelAddr := strings.TrimSpace(os.Getenv("PANOPTICON_PANEL_ADDR"))
	cert := strings.TrimSpace(os.Getenv("PANOPTICON_TLS_CERT_FILE"))
	firebase := strings.TrimSpace(os.Getenv("PANOPTICON_FIREBASE_CREDENTIALS"))

	chronicleState := "missing"
	if _, err := os.Stat(chronicle); err == nil {
		chronicleState = "present"
	}

	fmt.Printf("version        %s\n", version)
	fmt.Printf("chronicle      %s (%s)\n", chronicle, chronicleState)
	fmt.Printf("panel          %s\n", panelAddr)
	fmt.Printf("tls_cert       %s\n", cert)
	fmt.Printf("firebase       %s\n", firebase)
	fmt.Printf("service        %s\n", serviceState(serviceName))
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
		return fmt.Errorf("Panopticon is not installed at %s", installBinary)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	remoteVersion, err := fetchReleaseText(client, releaseBase+"/VERSION")
	if err != nil {
		return fmt.Errorf("recall remote Panopticon version: %w", err)
	}
	if remoteVersion == version {
		fmt.Printf("Panopticon is already %s\n", version)
		return nil
	}

	asset := "panopticon-linux-" + runtime.GOARCH
	checksums, err := fetchReleaseText(client, releaseBase+"/checksums.txt")
	if err != nil {
		return fmt.Errorf("recall Panopticon checksums: %w", err)
	}
	wantHash, err := checksumForAsset(checksums, asset)
	if err != nil {
		return err
	}

	payload, err := fetchReleaseBytes(client, releaseBase+"/"+asset)
	if err != nil {
		return fmt.Errorf("download Panopticon: %w", err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != wantHash {
		return fmt.Errorf("downloaded Panopticon checksum does not match")
	}

	if err := replaceExecutable(installBinary, payload); err != nil {
		return err
	}
	_ = runRootCommand("systemctl", "restart", serviceName)
	fmt.Printf("Panopticon updated from %s to %s\n", version, remoteVersion)
	return nil
}

func uninstall(args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	if _, err := os.Stat(installBinary); err != nil && os.IsNotExist(err) {
		if _, unitErr := os.Stat(installUnit); unitErr != nil && os.IsNotExist(unitErr) {
			return fmt.Errorf("Panopticon is not installed")
		}
	}

	if err := confirmDestructive(
		args,
		"This removes the Panopticon unit, binary, Chronicle, and user panopticon.\nType yes to continue: ",
	); err != nil {
		return err
	}

	_ = runRootCommand("systemctl", "stop", serviceName)
	_ = runRootCommand("systemctl", "disable", serviceName)
	if err := removePath(installUnit); err != nil {
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
	fmt.Println("Panopticon removed")
	return nil
}

func hashPanelPassword() error {
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimRight(string(payload), "\r\n")
	if password == "" {
		return fmt.Errorf("password is required")
	}
	encoded, err := encodeArgon2idPassword(password)
	if err != nil {
		return err
	}
	fmt.Println(encoded)
	return nil
}

func encodeArgon2idPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("forge password salt: %w", err)
	}

	const memory = 64 * 1024
	const iterations = 3
	const parallelism = 4
	const keyLength = 32

	key := argon2.IDKey(
		[]byte(password),
		salt,
		iterations,
		memory,
		parallelism,
		keyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
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
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "yes" && answer != "y" {
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
	tempFile, err := os.CreateTemp(directory, ".panopticon-*")
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
