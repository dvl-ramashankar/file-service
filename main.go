package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const maxUploadSize = 2 * 1024 * 1024 // 2 mb
const uploadPath = "./demo"
const downloadFileFromPath = "demo/"
const destination = "test/download/"

func main() {
	http.HandleFunc("/upload", uploadFileHandler())

	//	fs := http.FileServer(http.Dir(uploadPath))
	http.HandleFunc("/files/", downloadFiles()) //http.StripPrefix("/files", fs)

	log.Print("Server started on localhost:8080, use /upload for uploading files and /files/{fileName} for downloading")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func uploadFileHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			t, _ := template.ParseFiles("upload.gtpl")
			t.Execute(w, nil)
			return
		}
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			fmt.Printf("Could not parse multipart form: %v\n", err)
			renderError(w, "CANT_PARSE_FORM", http.StatusInternalServerError)
			return
		}

		// parse and validate file and post parameters
		file, fileHeader, err := r.FormFile("uploadFile")
		if err != nil {
			renderError(w, "INVALID_FILE", http.StatusBadRequest)
			return
		}
		defer file.Close()
		// Get and print out file size
		fileSize := fileHeader.Size
		fmt.Printf("File size (bytes): %v\n", fileSize)
		// validate file size
		if fileSize > maxUploadSize {
			renderError(w, "FILE_TOO_BIG", http.StatusBadRequest)
			return
		}
		fileBytes, err := ioutil.ReadAll(file)
		if err != nil {
			renderError(w, "INVALID_FILE", http.StatusBadRequest)
			return
		}

		// check file type, detectcontenttype only needs the first 512 bytes
		detectedFileType := http.DetectContentType(fileBytes)
		switch detectedFileType {
		case "image/jpeg", "image/jpg":
		case "image/gif", "image/png":
		case "application/pdf":
			break
		default:
			renderError(w, "INVALID_FILE_TYPE", http.StatusBadRequest)
			return
		}
		fileName := randToken(12)
		fileEndings, err := mime.ExtensionsByType(detectedFileType)
		if err != nil {
			renderError(w, "CANT_READ_FILE_TYPE", http.StatusInternalServerError)
			return
		}
		newPath := filepath.Join(uploadPath, fileName+fileEndings[0])
		fmt.Printf("FileType: %s, File: %s\n", detectedFileType, newPath)

		// write file
		newFile, err := os.Create(newPath)
		if err != nil {
			renderError(w, "CANT_WRITE_FILE", http.StatusInternalServerError)
			return
		}
		defer newFile.Close() // idempotent, okay to call twice
		if _, err := newFile.Write(fileBytes); err != nil || newFile.Close() != nil {
			renderError(w, "CANT_WRITE_FILE", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("SUCCESS"))
	})
}

func renderError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(message))
}

func randToken(len int) string {
	b := make([]byte, len)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

var (
	fileName    string
	fullURLFile string
)

func downloadFiles() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if r.Method != "GET" {
			renderError(w, "Method not allowed", http.StatusInternalServerError)
			return
		}

		fileName := strings.Split(r.URL.Path, "/")[2]
		fullURLFile = downloadFileFromPath + fileName

		// Build fileName from fullPath
		fileURL, err := url.Parse(fullURLFile)
		if err != nil {
			log.Fatal(err)
			renderError(w, "Path_Not_Available", http.StatusInternalServerError)
			return
		}
		path := fileURL.Path
		segments := strings.Split(path, "/")
		fileName = segments[len(segments)-1]
		fileName = destination + fileName
		// Create blank file
		file, err := os.Create(fileName)
		if err != nil {
			log.Println(err)
			renderError(w, "Unable_To_Create_File", http.StatusInternalServerError)
			return
		}

		resp, err := os.Open(fullURLFile)
		if err != nil {
			log.Println(err)
			renderError(w, "File_Not_Available", http.StatusInternalServerError)
			return
		}
		defer resp.Close()
		size, err := io.Copy(file, resp)
		defer file.Close()
		fmt.Printf("Downloaded a file %s with size %d", fileName, size)
		respondWithJson(w, http.StatusOK, "Downloaded Successfully")
	})
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJson(w, code, map[string]string{"error": msg})
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
