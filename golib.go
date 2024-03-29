package golib

import (
	// "encoding/json"
	// "fmt"
	// "github.com/gorilla/mux"
	// "io"
	// "log"
	"net/http"
	// "os"
	// "strings"
	// "strconv"
    "crypto/subtle"
    // "io/ioutil"
    // "regexp"

    // "github.com/davecgh/go-spew/spew"
)

func BasicAuth(handler http.HandlerFunc, username, password, realm string) http.HandlerFunc {

    return func(w http.ResponseWriter, r *http.Request) {

        user, pass, ok := r.BasicAuth()

        if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
            w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
            w.WriteHeader(401)
            w.Write([]byte("Unauthorised.\n"))
            return
        }

        handler(w, r)
    }
}