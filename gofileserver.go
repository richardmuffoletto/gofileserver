package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "net/http"
    "strconv"

    "github.com/gorilla/mux"

    "rpm/gofileserver/internal/auth"
    "rpm/gofileserver/internal/files"
)


const MAX_JSON_PAYLOAD = 1024
const MAX_FILE_UPLOAD = 1024*1024


type ErrorResponse struct {
    Error  string  `json:"error"`
}

type LoginResponse struct {
    AccessToken  string  `json:"token"`
}


func main() {
    fmt.Println("starting file server...")

    // setup router
    router := mux.NewRouter().StrictSlash(true)

    router.Methods("POST").Path("/register").HandlerFunc(doRegister)

    router.Methods("POST").Path("/login").HandlerFunc(doLogin)

    router.PathPrefix("/files").Handler(http.StripPrefix("/files", http.HandlerFunc(doFiles)))

    log.Fatal(http.ListenAndServe(":8080", router))
}


// Helper function that sets the Content-Type to json.
func setJsonContentType(w http.ResponseWriter) {
    w.Header().Set("Content-Type", "application/json; charset=UTF-8")
}


// Helper function that returns the statusCode and encodes the given obj if not nil.
func jsonResponse(w http.ResponseWriter, statusCode int, obj interface{}) {
    // NOTE the order below matters
    if obj != nil {
        setJsonContentType(w)
    }
    w.WriteHeader(statusCode)
    if obj != nil {
        json.NewEncoder(w).Encode(obj)
    }
}


// Helper function to return an error message encoded in json.
func jsonErrorResponse(w http.ResponseWriter, statusCode int, errorMessage string) {
    jsonResponse(w, statusCode, &ErrorResponse{Error: errorMessage})
}


// Helper function to parse the request body as json and populate the given object.
// If parsing fails, an error message and statusCode are written to the response.
func parseJsonBody(w http.ResponseWriter, r *http.Request, obj interface{}) error {

    body, err := ioutil.ReadAll(io.LimitReader(r.Body, MAX_JSON_PAYLOAD))
    if err != nil {
        jsonErrorResponse(w, http.StatusInternalServerError, "Failed reading request body")
        return err
    }
    if err := r.Body.Close(); err != nil {
        jsonErrorResponse(w, http.StatusInternalServerError, "Failed closing request body")
        return err
    }

    if err := json.Unmarshal(body, obj); err != nil {
        jsonErrorResponse(w, http.StatusBadRequest, "Failed parsing JSON request")
        return err
    }

    return nil
}


// Register a new user.
func doRegister(w http.ResponseWriter, r *http.Request) {

    var user auth.User
    if err := parseJsonBody(w, r, &user); err != nil {
        return
    }
    
    if err := auth.CreateUser(&user); err != nil {
        jsonErrorResponse(w, http.StatusBadRequest, err.Error())
        return
    }

    jsonResponse(w, http.StatusNoContent, nil)
}


// Login a user, returning an authentication token for later calls.
func doLogin(w http.ResponseWriter, r *http.Request) {

    var user auth.User
    if err := parseJsonBody(w, r, &user); err != nil {
        return
    }

    accessToken, err := auth.Login(&user)
    if err != nil {
        jsonErrorResponse(w, http.StatusForbidden, err.Error())
        return
    }

    jsonResponse(w, http.StatusOK, &LoginResponse{AccessToken: accessToken})
}


// Dispatches file requests for a given user, determined by the authentication token.
func doFiles(w http.ResponseWriter, r *http.Request) {

    path := r.URL.Path

    // trim off leading slash if present
    if len(path) > 0 {
        path = path[1:]
    }

    userID, err := auth.ValidateToken(r.Header.Get("X-Session"))
    if err != nil {
        w.WriteHeader(http.StatusForbidden) // 403
        return
    }

    // dispatch based on method
    if r.Method == "GET" {
        if path == "" {
            doFilesList(userID, w, r)
            return
        }
        doFilesGet(userID, path, w, r)
        return
    }
    if r.Method == "PUT" {
        doFilesPut(userID, path, w, r)
        return
    }
    if r.Method == "DELETE" {
        doFilesDelete(userID, path, w, r)
        return
    }

    w.WriteHeader(http.StatusMethodNotAllowed) // 405
}


// Return the list of files, as an array of string filenames, for the user.
func doFilesList(userID string, w http.ResponseWriter, r *http.Request) {

    files := files.ListFilenames(userID)

    jsonResponse(w, http.StatusOK, &files)
}


// Return the body of the given file.
func doFilesGet(userID string, path string, w http.ResponseWriter, r *http.Request) {

    fileBytes, contentType, present := files.GetFile(userID, path)

    if !present {
        jsonResponse(w, http.StatusNotFound, nil)
        return
    }

    // NOTE: Content-Length is set automatically

    w.Header().Set("Content-Type", contentType)
    w.WriteHeader(http.StatusOK) // 200

    io.Copy(w, bytes.NewReader(fileBytes))
}


// Upload a file into a user's space, possibly overwriting an existing file with the same path.
func doFilesPut(userID string, path string, w http.ResponseWriter, r *http.Request) {

    contentType := r.Header.Get("Content-Type")

    contentLength, err := strconv.Atoi(r.Header.Get("Content-Length"))
    if err != nil {
        jsonErrorResponse(w, http.StatusBadRequest, "Content-Length required")
        return
    }
    if contentLength == 0 {
        jsonErrorResponse(w, http.StatusBadRequest, "Content-Length required")
        return
    }
    if contentLength > MAX_FILE_UPLOAD {
        jsonErrorResponse(w, http.StatusBadRequest, "Content-Length exceeds maximum")
        return
    }

    bodyBytes, err := ioutil.ReadAll(io.LimitReader(r.Body, MAX_FILE_UPLOAD))
    if err != nil {
        jsonErrorResponse(w, http.StatusBadRequest, "Failed reading file upload")
        return
    }
    if err := r.Body.Close(); err != nil {
        jsonErrorResponse(w, http.StatusInternalServerError, "Failed closing body")
        return
    }

    if contentLength != len(bodyBytes) {
        jsonErrorResponse(w, http.StatusBadRequest, "Content-Length does not match uploaded file")
        return
    }

    if err := files.PutFile(userID, path, contentType, bodyBytes); err != nil {
        jsonErrorResponse(w, http.StatusBadRequest, "Failed saving file")
        return
    }

    jsonResponse(w, http.StatusCreated, nil)
}


// Delete the given file for the user.
func doFilesDelete(userID string, path string, w http.ResponseWriter, r *http.Request) {

    if err := files.DeleteFile(userID, path); err != nil {
        jsonErrorResponse(w, http.StatusNotFound, "file not found")
        return
    }

    jsonResponse(w, http.StatusNoContent, nil)
}

