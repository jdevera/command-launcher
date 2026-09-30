package helper

import (
	"fmt"
	"io"
	"net/http"
)

func HttpGetWithBasicAuth(url, user, password string) (int, []byte, error) {
	return HttpDoWithBasicAuth("GET", url, user, password, nil)
}

func HttpPostWithBasicAuth(url, user, password string) (int, []byte, error) {
	return HttpDoWithBasicAuth("POST", url, user, password, nil)
}

func HttpPostInputWithBasicAuth(url, user, password string, input io.Reader) (int, []byte, error) {
	return HttpDoWithBasicAuth("POST", url, user, password, input)
}

func HttpDoWithBasicAuth(method, url, user, password string, input io.Reader) (int, []byte, error) {
	req, err := http.NewRequest(method, url, input)
	if err != nil {
		return 0, nil, err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}

	req.SetBasicAuth(user, password)
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	if !Is2xx(resp.StatusCode) {
		return resp.StatusCode, nil, fmt.Errorf("failed to get resources %s, status code %d", url, resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, body, nil
}

func BodyAsString(resp *http.Response) (string, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
