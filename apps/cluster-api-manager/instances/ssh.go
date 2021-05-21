package instances

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func GetGitHubUserSSHKeys(username string) (sshKeys []string, err error) {
	resp, err := http.Get(fmt.Sprintf("https://github.com/%s.keys", username))
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()
	sshKeysBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{}, err
	}
	sshKeysString := strings.Trim(string(sshKeysBytes), "\n")
	sshKeys = strings.Split(sshKeysString, "\n")
	return sshKeys, err
}
