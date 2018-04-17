package auth

import (
    "crypto/sha256"
    "errors"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "regexp"
    "time"
    
    "github.com/google/uuid"

    bolt "github.com/coreos/bbolt"
)


const DB_NAME = "gofileserver_auth.db"
const USER_BUCKET = "UserBucket"
const TOKEN_BUCKET = "TokenBucket"


type User struct {
    ID       string  `json:"id"`
    Username string  `json:"username"`
    Password string  `json:"password"`
}

type Token struct {
    AccessToken  string     `json:"token"`
    UserID       string     `json:"user"`
    Created      time.Time  `json:"created"`
}


func init() {

    // Create database and buckets for Users and Tokens

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

    _, err = tx.CreateBucketIfNotExists([]byte(USER_BUCKET))
    if err != nil {
        panic(err)
    }

    _, err = tx.CreateBucketIfNotExists([]byte(TOKEN_BUCKET))
    if err != nil {
        panic(err)
    }

    if err = tx.Commit(); err != nil {
        panic(err)
    }

    db.Close()
}


// Helper function that computes the hex-encoded SHA256 value for a password.
func hashPassword(password string) string {
    //TODO: hash password with salt?
    sum := sha256.Sum256([]byte(password))
    return hex.EncodeToString(sum[:])
}


// Creates a user from the given object.
// Validates the Username: between 3 and 20 alphanumeric characters.
// Validates the Password: at least 8 characters.
// The ID field will be overwritten.
func CreateUser(user *User) error {

    //Validate fields

    //Username must be between 3 and 20 alphanumeric characters
    invalidCharsRegex := regexp.MustCompile("[^A-Za-z0-9]+")
    if len(user.Username) < 3 || len(user.Username) > 20 || invalidCharsRegex.MatchString(user.Username) {
        return errors.New("username must be 3 to 20 alphanumeric characters")        
    }

    //Passwords must be at least 8 characters
    if len(user.Password) < 8 {
        return errors.New("password must be at least 8 characters")
    }

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    err = db.Update(func(tx *bolt.Tx) error {

        b := tx.Bucket([]byte(USER_BUCKET))

        //ensure username is not taken
        v := b.Get([]byte(user.Username))
        if v != nil {
            return errors.New("username already taken")
        }

        // generate UUID that never changes for this user.
        user.ID = uuid.New().String()

        // hash password (NOTE this will modify the user object passed in)
        user.Password = hashPassword(user.Password)

        // encode to JSON
        encoded, err := json.Marshal(user)
        if err != nil {
            return err
        }

        // put in database
        err = b.Put([]byte(user.Username), encoded)

        return err //nil implies commit transaction, otherwise rollback
    })

    return err
}


// Login a user, generating a token for the session.
func Login(user *User) (accessToken string, err error) {
    fmt.Printf("Login...")

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    err = db.Update(func(tx *bolt.Tx) error {
        userBucket := tx.Bucket([]byte(USER_BUCKET))
        

        //retrieve user from database
        v := userBucket.Get([]byte(user.Username))
        if v == nil {
            return errors.New("authentication failed") //TODO: create constant somewhere for this
        }

        var dbUser User
        if err := json.Unmarshal(v, &dbUser); err != nil {
            return errors.New("authentication failed (failed parsing JSON from database)")
        }

        // compare hashed password
        if dbUser.Password != hashPassword(user.Password) {
            return errors.New("authentication failed")
        }

        accessToken = uuid.New().String()

        token := Token{
            AccessToken: accessToken,
            UserID: dbUser.ID,
            Created: time.Now(),
        }

        // encode Token to JSON
        encodedToken, err := json.Marshal(&token)
        if err != nil {
            return err
        }

        // store token in database
        tokenBucket := tx.Bucket([]byte(TOKEN_BUCKET))
        err = tokenBucket.Put([]byte(accessToken), encodedToken)

        return err
    })

    if err != nil {
        accessToken = ""
    }

    return accessToken, err
}


// Validates a given authentication token, returning the appropriate user for that token.
func ValidateToken(accessToken string) (userID string, err error) {

    if accessToken == "" {
        return "", errors.New("invalid accessToken")
    }

    db, err := bolt.Open(DB_NAME, 0600, nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()

    err = db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte(TOKEN_BUCKET))

        v := b.Get([]byte(accessToken))
        if v == nil {
            return errors.New("invalid accessToken")
        }

        var token Token
        if err := json.Unmarshal(v, &token); err != nil {
            return errors.New("invalid accessToken (failed parsing JSON from database)")
        }

        //TODO: validate TTL of token

        userID = token.UserID

        return nil
    })

    if err != nil {
        userID = ""
    }

    return userID, err
}

