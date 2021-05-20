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
	sshKey, err := io.ReadAll(resp.Body)
	sshKeys = strings.Split(string(sshKey), "\n")
	return sshKeys, err
}
