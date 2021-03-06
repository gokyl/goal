package goal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
)

// validateCols columns are valid
func validateCols(usernameCol string, passwordCol string, user interface{}) error {
	// validateCols column names
	scope := db.NewScope(user)
	cols := []string{usernameCol, passwordCol}
	for _, col := range cols {
		if !scope.HasColumn(col) {
			errorMsg := fmt.Sprintf("Column %s does not exist", col)
			return errors.New(errorMsg)
		}
	}

	return nil
}

// RegisterWithPassword checks if username exists and
// sets password with bcrypt algorithm
// Client can provides extra data to be saved into database for user
func RegisterWithPassword(
	w http.ResponseWriter, request *http.Request,
	usernameCol string, passwordCol string) (interface{}, error) {
	if request.Method != POST {
		return nil, http.ErrNotSupported
	}

	user, err := getUserResource()
	if err != nil {
		return nil, err
	}

	// Parse request body into resource
	decoder := json.NewDecoder(request.Body)
	var values map[string]string
	err = decoder.Decode(&values)
	if err != nil {
		return nil, err
	}

	username := values[usernameCol]
	password := values[passwordCol]

	if username == "" || password == "" {
		return nil, errors.New("username or password is not found")
	}

	err = validateCols(usernameCol, passwordCol, user)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Search db, if a username is already defined, return error
	qryStr := fmt.Sprintf("%s = ?", usernameCol)
	var count int
	qry := db.Where(qryStr, username).First(user).Count(&count)
	err = qry.Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	if count > 0 {
		return nil, errors.New("account already exists")
	}

	// Since user was populated with extra data, we need to
	// setup new scope
	scope := db.NewScope(user)

	// Save a new record to db
	scope.SetColumn(usernameCol, username)

	// Hashing the password with the default cost of 10
	hashedPw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	scope.SetColumn(passwordCol, hashedPw)
	err = scope.DB().New().Create(scope.Value).Error
	if err != nil {
		return nil, err
	}

	// Set current session
	SetUserSession(w, request, user)

	return user, nil
}

// LoginWithPassword checks if username and password correct
// and set user into session
func LoginWithPassword(
	w http.ResponseWriter, request *http.Request,
	usernameCol string, passwordCol string) (interface{}, error) {
	if request.Method != POST {
		return nil, http.ErrNotSupported
	}

	user, err := getUserResource()
	if err != nil {
		return nil, err
	}

	err = validateCols(usernameCol, passwordCol, user)
	if err != nil {
		return nil, err
	}

	// Parse request body into resource
	decoder := json.NewDecoder(request.Body)
	var values map[string]string
	err = decoder.Decode(&values)
	if err != nil {
		return nil, err
	}

	username := values[usernameCol]
	password := values[passwordCol]

	if username == "" || password == "" {
		return nil, errors.New("username or password is not found")
	}

	// Search db, if a username is not found, return error
	qry := fmt.Sprintf("%s = ?", usernameCol)

	qryDB := db.Where(qry, username).First(user)
	err = qryDB.Error
	if err != nil {
		return nil, err
	}

	if user == nil {
		errorMsg := fmt.Sprintf("Username not found: %s", username)
		return nil, errors.New(errorMsg)
	}

	// Make sure the password is correct
	var hashs []string
	qryDB.Pluck(passwordCol, &hashs)

	if len(hashs) == 0 {
		errorMsg := fmt.Sprintf("Unable to get value from column: %s", passwordCol)
		return nil, errors.New(errorMsg)
	}

	hashed := hashs[0]

	// Comparing the password with the hash
	err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	if err != nil {
		return nil, err
	}

	// Set current session
	SetUserSession(w, request, user)

	return user, nil
}

// HandleLogout let user logout from the system
func HandleLogout(w http.ResponseWriter, request *http.Request) {
	ClearUserSession(w, request)
}
