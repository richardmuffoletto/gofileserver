package files

import (
    "encoding/json"
    "errors"
    "fmt"

    "github.com/google/uuid"
    
    bolt "github.com/coreos/bbolt"
)


const DB_NAME = "gofileserver_files.db"
const USERDATA_BUCKET = "UserFileBucket"
const FILES_BUCKET = "FileBucket"


type FileMetadata struct {
    ID            string  `json:"id"`
    ContentType   string  `json:"contentType"`
    ContentLength int     `json:"contentLength"`
}

type UserData struct {
    Files  map[string]FileMetadata   `json:"files"`
}


func init() {

    // Create database and buckets for Files and UserFiles

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }

    //TODO: consider refactoring into a tx in a closure

    tx, err := db.Begin(true)
    if err != nil {
        panic(err)
    }
    defer tx.Rollback()

    _, err = tx.CreateBucketIfNotExists([]byte(USERDATA_BUCKET))
    if err != nil {
        panic(err)
    }

    _, err = tx.CreateBucketIfNotExists([]byte(FILES_BUCKET))
    if err != nil {
        panic(err)
    }

    if err = tx.Commit(); err != nil {
        panic(err)
    }

    db.Close()
}


// Returns an array of filenames as strings for the given user.
func ListFilenames(userID string) (filenames []string) {

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()


    err = db.View(func(tx *bolt.Tx) error {
        userFilesBucket := tx.Bucket([]byte(USERDATA_BUCKET))

        var userData UserData

        v := userFilesBucket.Get([]byte(userID))
        if v == nil {
            //no user implies no file either
            return errors.New("user not found")
        }
        //user has an entry
        if err := json.Unmarshal(v, &userData); err != nil {
            fmt.Printf("Failed parsing JSON for UserData")
            return err
        }

        // iterate over files in metadata, appending to list
        for filename := range userData.Files {
            filenames = append(filenames, filename)
        }

        return nil
    })

    if filenames == nil {
        filenames = make([]string, 0)
    }

    return filenames
}


// Feturns, for the given filename in the user's space, the raw bytes and saved contentType value.
func GetFile(userID string, filename string) (bytes []byte, contentType string, present bool) {

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    contentType = ""

    err = db.View(func(tx *bolt.Tx) error {
        userFilesBucket := tx.Bucket([]byte(USERDATA_BUCKET))

        var userData UserData

        v := userFilesBucket.Get([]byte(userID))
        if v == nil {
            //no user implies no file either
            return errors.New("user not found")
        }
        //user has an entry
        if err := json.Unmarshal(v, &userData); err != nil {
            fmt.Printf("Failed parsing JSON for UserData")
            return err
        }

        fileMetadata, present := userData.Files[filename]

        if !present {
            return errors.New("file not found")
        }

        filesBucket := tx.Bucket([]byte(FILES_BUCKET))

        bucketBytes := filesBucket.Get([]byte(fileMetadata.ID))

        //copy bytes...
        bytes = make([]byte, len(bucketBytes))
        copy(bytes, bucketBytes)

        contentType = fileMetadata.ContentType

        return nil
    })

    present = (err == nil)
    return bytes, contentType, present
}


// Saves the given file into the user's space.
func PutFile(userID string, filename string, contentType string, bytes []byte) error {

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    err = db.Update(func(tx *bolt.Tx) error {

        userFilesBucket := tx.Bucket([]byte(USERDATA_BUCKET))

        var userData UserData

        v := userFilesBucket.Get([]byte(userID))
        if v != nil {
            //user already has an entry
            if err := json.Unmarshal(v, &userData); err != nil {
                fmt.Printf("Failed parsing JSON for UserData")
                return err
            }
        } else {
            // initialize map of Files
            userData.Files = make(map[string]FileMetadata)
        }

        fileMetadata, present := userData.Files[filename]
        if present {
            fileMetadata.ContentType = contentType
            fileMetadata.ContentLength = len(bytes)
            userData.Files[filename] = fileMetadata
        } else {
            userData.Files[filename] = FileMetadata{
                ID: uuid.New().String(),
                ContentType: contentType,
                ContentLength: len(bytes),
            }
        }

        // encode to JSON
        var encoded []byte
        encoded, err = json.Marshal(userData)
        if err != nil {
            return err
        }

        // put in database
        err = userFilesBucket.Put([]byte(userID), encoded)
        if err != nil {
            return err
        }

        //insert bytes into FilesBucket
        filesBucket := tx.Bucket([]byte(FILES_BUCKET))
        err = filesBucket.Put([]byte(userData.Files[filename].ID), bytes)
        if err != nil {
            return err
        }

        return nil
    })

    return err
}


// Deletes the given file from the user's space.
func DeleteFile(userID string, filename string) error {

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    err = db.Update(func(tx *bolt.Tx) error {

        userFilesBucket := tx.Bucket([]byte(USERDATA_BUCKET))

        var userData UserData

        v := userFilesBucket.Get([]byte(userID))
        if v == nil {
            // no user, can't delete file
            return nil
        }
        if err := json.Unmarshal(v, &userData); err != nil {
            fmt.Printf("Failed parsing JSON for UserData")
            return err
        }

        fileMetadata, present := userData.Files[filename]
        if !present {
            return errors.New("file not found")
        }

        // save fileID to delete from FilesBucket
        fileID := fileMetadata.ID
        delete(userData.Files, filename)

        // encode to JSON
        var encoded []byte
        encoded, err = json.Marshal(userData)
        if err != nil {
            return err
        }

        // update database
        err = userFilesBucket.Put([]byte(userID), encoded)
        if err != nil {
            return err
        }

        // delete actual file data from FilesBucket
        filesBucket := tx.Bucket([]byte(FILES_BUCKET))
        err = filesBucket.Delete([]byte(fileID))
        if err != nil {
            return err
        }

        return nil
    })

    return err
}



