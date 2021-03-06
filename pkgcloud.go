// Package pkgcloud allows you to talk to the packagecloud API.
// See https://packagecloud.io/docs/api
package pkgcloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/mlafeldt/pkgcloud/upload"
)

//go:generate bash -c "./gendistros.py supportedDistros | gofmt > distros.go"

// ServiceURL is the URL of packagecloud's API.
const ServiceURL = "https://packagecloud.io/api/v1"

// A Client is a packagecloud client.
type Client struct {
	token string
}

// NewClient creates a packagecloud client. API requests are authenticated
// using an API token. If no token is passed, it will be read from the
// PACKAGECLOUD_TOKEN environment variable.
func NewClient(token string) (*Client, error) {
	if token == "" {
		token = os.Getenv("PACKAGECLOUD_TOKEN")
		if token == "" {
			return nil, errors.New("PACKAGECLOUD_TOKEN unset")
		}
	}
	return &Client{token}, nil
}

func decodeResponse(status int, body []byte) error {
	switch status {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusUnauthorized, http.StatusNotFound:
		return fmt.Errorf("HTTP status: %s", http.StatusText(status))
	case 422: // Unprocessable Entity
		var v map[string][]string
		if err := json.Unmarshal(body, &v); err != nil {
			return err
		}
		for _, messages := range v {
			for _, msg := range messages {
				// Only return the very first error message
				return errors.New(msg)
			}
			break
		}
		return fmt.Errorf("invalid HTTP body: %s", body)
	default:
		return fmt.Errorf("unexpected HTTP status: %d", status)
	}
}

// CreatePackage pushes a new package to packagecloud.
func (c Client) CreatePackage(repo, distro, pkgFile string) error {
	var extraParams map[string]string
	if distro != "" {
		distID, ok := supportedDistros[distro]
		if !ok {
			return fmt.Errorf("invalid distro name: %s", distro)
		}
		extraParams = map[string]string{
			"package[distro_version_id]": strconv.Itoa(distID),
		}
	}

	endpoint := fmt.Sprintf("%s/repos/%s/packages.json", ServiceURL, repo)
	request, err := upload.NewRequest(endpoint, extraParams, "package[package_file]", pkgFile)
	if err != nil {
		return err
	}
	request.SetBasicAuth(c.token, "")
	request.Header.Add("User-Agent", "pkgcloud Go client")

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return decodeResponse(resp.StatusCode, body)
}
