package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	fileNamePattern = regexp.MustCompile("[^a-zA-Z0-9]+")
)

type bCup struct {
	ID        string
	GroupID   string
	GroupName string
	Title     string
	Username  string
	Password  string
	URL       *url.URL
	Notes     string
}

type gopassJSONOut struct {
	Name           string `json:"entry_name"`
	Login          string `json:"login"`
	Password       string `json:"password"`
	PasswordLength int    `json:"length"`
	Generate       bool   `json:"generate"`
	UseSymbols     bool   `json:"use_symbols"`
}

func main() {
	fileName := flag.String("file", "", "File name to read")
	storePath := flag.String("storePath", "~/.password-store/", "Password store base path")
	dryRun := flag.Bool("dryrun", false, "Dry run")
	flag.Parse()

	if *fileName == "" {
		log.Fatalln("missing file")
	}

	f, err := os.Open(*fileName)
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	items := make([]bCup, 0)
	cr := csv.NewReader(f)
	_, _ = cr.Read() // Ignore the header

	for {
		rec, err := cr.Read()
		if err != nil {
			if err != io.EOF {
				log.Fatalln(err)
			}
			break
		}

		u, err := url.Parse(rec[5])
		if err != nil {
			u = &url.URL{
				Host: rec[5],
			}
		}
		items = append(items, bCup{ // [!group_id !group_name title username password URL Notes !group_id !group_name id]
			ID:        rec[9],
			GroupID:   rec[0],
			GroupName: strings.ToLower(rec[1]),
			Title:     rec[2],
			Username:  rec[3],
			Password:  rec[4],
			URL:       u,
			Notes:     rec[6],
		})
	}

	pgpKeyID, err := readKeyID(normalizePath(*storePath))
	if err != nil {
		log.Fatalln(err)
	}

	//for _, i := range items {
	//	fmt.Println(passOutput(i))
	//}
	//
	gpgPath, err := detectGpgBinary()
	if err != nil {
		log.Fatalln(err)
	}

	for _, i := range items {

		if *dryRun {
			log.Println("Would create:", filepath.Join(normalizePath(*storePath), genPassFilePath(i)), "\nFile:", genPassFileName(i), "\n", genPassContent(i))
			continue
		}

		err = encryptData(gpgPath, pgpKeyID, filepath.Join(normalizePath(*storePath), genPassFilePath(i)), genPassFileName(i), genPassContent(i))
		if err != nil {
			log.Fatalln(err)
		}
		log.Println("created:", filepath.Join(normalizePath(*storePath), genPassFilePath(i), genPassFileName(i)))
	}
}

func genPassFilePath(item bCup) string {
	if item.URL.Host != "" {
		return item.GroupName + "/" + item.URL.Host
	}
	return item.GroupName + "/" + convertToFileName(item.Title)
}

func genPassFileName(item bCup) string {
	if item.Username != "" {
		return item.Username + ".gpg"
	}

	return convertToFileName(item.Title) + ".gpg"
}

func isFileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return false // maybe or may not
}

func convertToFileName(name string) string {
	name = strings.ToLower(name)
	name = strings.Replace(name, " ", "_", -1)
	fileNamePattern.ReplaceAllString(name, "")
	return name
}

func genPassContent(item bCup) string {
	var sb strings.Builder

	//sb.WriteString(item.GroupName+"/"+item.URL.Host+"/"+item.Username+":")
	//sb.WriteString("\n")

	sb.WriteString(item.Password)
	sb.WriteString("\n")

	sb.WriteString("title: ")
	sb.WriteString(item.Title)
	sb.WriteString("\n")

	sb.WriteString("password: ")
	sb.WriteString(item.Password)
	sb.WriteString("\n")

	sb.WriteString("login: ")
	sb.WriteString(item.Username)
	sb.WriteString("\n")

	sb.WriteString("url: ")
	sb.WriteString(item.URL.String())
	sb.WriteString("\n")

	sb.WriteString("group: ")
	sb.WriteString(item.GroupName)
	sb.WriteString("\n")

	sb.WriteString("comments: ")
	sb.WriteString(item.Notes)
	sb.WriteString("\n")

	return sb.String()
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		usr, _ := user.Current()
		path = filepath.Join(usr.HomeDir, path[2:])
	}
	return path
}

func readKeyID(storePath string) (string, error) {
	id, err := ioutil.ReadFile(filepath.Join(storePath, ".gpg-id"))
	return strings.TrimSpace(string(id)), err
}

func detectGpgBinary() (string, error) {
	// Look in $PATH first, then check common locations - the first successful result wins
	gpgBinaryPriorityList := []string{
		"gpg2", "gpg",
		"/bin/gpg2", "/usr/bin/gpg2", "/usr/local/bin/gpg2",
		"/bin/gpg", "/usr/bin/gpg", "/usr/local/bin/gpg",
	}

	for _, binary := range gpgBinaryPriorityList {
		err := validateGpgBinary(binary)
		if err == nil {
			return binary, nil
		}
	}
	return "", fmt.Errorf("unable to detect the location of the gpg binary to use")
}

func validateGpgBinary(gpgPath string) error {
	return exec.Command(gpgPath, "--version").Run()
}

func createFolders(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

func encryptData(gpgPath string, keyID string, filePath string, fileName string, content string) error {
	fileWithPath := filepath.Join(filePath, fileName)
	if err := createFolders(filepath.Dir(fileWithPath)); err != nil {
		return err
	}

	var stdout, stderr bytes.Buffer


	idx := 1
	for {
		if isFileExists(fileWithPath) {
			fileWithPath = fileWithPath + fmt.Sprintf("_%d", idx)
			fmt.Println("File exists, try indexing:", fileWithPath)
		} else {
			break
		}
		idx++
	}

	//gpgOptions := []string{"--encrypt", "--yes", "--recipient", keyID, "--output", filepath.Join(filePath, fileName)}
	gpgOptions := []string{"--encrypt", "--recipient", keyID, "--output", fileWithPath}

	cmd := exec.Command(gpgPath, gpgOptions...)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error: %s, Stderr: %s", err.Error(), stderr.String())
	}

	return nil
}
