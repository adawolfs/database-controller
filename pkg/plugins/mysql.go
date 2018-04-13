package plugins

import (
	"log"
	"fmt"
	"strings"
	"net/url"
	"database/sql"
	"github.com/sethvargo/go-password/password"
	"github.com/adawolfs/database-controller/pkg/config"
	adawolfs "github.com/adawolfs/database-controller/pkg/apis/adawolfs.com/v1"
)

type mysqlHandler struct{
	config	config.Database
}

func NewMysqlHandlers(config config.Database) *mysqlHandler{
	handler := &mysqlHandler{
		config: config,

	}
	return handler
}

func (h *mysqlHandler) AddDatabase(db *adawolfs.Database) {

	u, err := url.Parse(h.config.URL)
	sPassword, _ := u.User.Password()

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/",
		u.User.Username(),
		sPassword,
		u.Host)
	dbconn, err := sql.Open("mysql", dsn)

	if err != nil {
		log.Printf("%s/%s: database connection failed: %v\n",
			db.Namespace, db.Name, err)
		//cllr.setError(db, fmt.Sprintf("database connection failed: %v", err))
		return
	}
	// After execution close connection
	defer dbconn.Close()

	// The database name will be <NAMESPACE>_<DB_RESOURCE_NAME>
	dbName := strings.Replace(fmt.Sprintf("%s_%s", db.Namespace, db.Name),"-", "_", -1)

	// Fills the db uer name
	dbUser := db.Spec.User
	if db.Spec.User == "" {
		dbUser = dbName
	}

	dbPassword := db.Spec.Password
	if db.Spec.Password == "" {
		dbPassword, err = password.Generate(16, 8, 0, false, true)
		if err != nil {
			//cllr.setError(db, "failed to generate password")
			log.Printf("%s/%s: failed to generate password: %s", db.Namespace, db.Name, err)
			return
		}
	}

	dbCharacterSet := db.Spec.Encoding
	if dbCharacterSet == "" {
		dbCharacterSet = "utf8"
	}

	dbCollate := db.Spec.Collate
	if dbCollate == "" {
		dbCollate = "utf8_general_ci"
	}

	//Creates Database
	_, err = dbconn.Exec(fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET %s COLLATE %s",
		dbName, dbCharacterSet, dbCollate))
	//If create database fails
	if err != nil {
		//cllr.setError(db, fmt.Sprintf("failed to create database: %v", err))
		log.Printf("%s/%s: failed to create database: %v\n",	db.Namespace, db.Name, err)
		return
	}

	//Gives permissions to database
	_, err = dbconn.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO `%s`@`%%` IDENTIFIED BY \"%s\"",
		strings.Replace(dbName, "_", "\\_", -1), dbUser, dbPassword))
	//If give permissions fails
	if err != nil {
		//cllr.setError(db, fmt.Sprintf("failed to create user: %v", err))
		log.Printf("%s/%s: failed to create user: %v\n", db.Namespace, db.Name, err)
		return
	}

}

func (h *mysqlHandler) DeleteDatabase(db *adawolfs.Database) {

	u, err := url.Parse(h.config.URL)
	sPassword, _ := u.User.Password()

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/",
		u.User.Username(),
		sPassword,
		u.Host)
	dbconn, err := sql.Open("mysql", dsn)

	if err != nil {
		log.Printf("%s/%s: database connection failed: %v\n",
			db.Namespace, db.Name, err)
		//cllr.setError(db, fmt.Sprintf("database connection failed: %v", err))
		return
	}
	// After execution close connection
	defer dbconn.Close()

	// The database name will be <NAMESPACE>_<DB_RESOURCE_NAME>
	dbName := strings.Replace(fmt.Sprintf("%s_%s", db.Namespace, db.Name),"-", "_", -1)

	_, err = dbconn.Exec(fmt.Sprintf("DROP DATABASE `%s`", dbName))
	if err != nil {
		log.Printf("%s/%s: failed to drop database \"%s\": %v\n",
			db.Namespace, db.Name, dbName, err)
		return
	}

	_, err = dbconn.Exec(fmt.Sprintf("DROP USER `%s`@`%%`", dbName))
	if err != nil {
		log.Printf("%s/%s: failed to drop user \"%s\": %v\n",
			db.Namespace, db.Name, err)
		return
	}
}

func (h *mysqlHandler) UpdateDatabase(db *adawolfs.Database) {

}