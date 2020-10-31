// db
package main

import (
	"database/sql"
	"errors"
	"log"
	"strconv"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
)

var groupChatId int64 = 0

type DatabaseAdapter struct {
	dbFileName string
	DB         *sql.DB
}

func (adapter *DatabaseAdapter) Close() {
	adapter.DB.Close()
}

func (adapter *DatabaseAdapter) connectSqlite(dbName string) {
	TABLES := []string{
		"users (id INTEGER PRIMARY KEY, username VARCHAR(64), password_hash VARCHAR(255));",

		`messages 
		(id INTEGER PRIMARY KEY, 
		text TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		chat_id INTEGER NOT NULL,
		created_on INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id));`}

	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		panic(err)
	}

	// Create initial tables
	for _, tableData := range TABLES {
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS " + tableData)
		if err != nil {
			panic(err)
		}
	}
	adapter.dbFileName = dbName
	adapter.DB = db
}

func (adapter *DatabaseAdapter) getAllUsers() []User {
	db := adapter.DB
	result := []User{}

	usersSql := sq.Select("*").From("users")
	rows, err := usersSql.RunWith(db).Query()
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		user := User{}
		err := rows.Scan(&user.Id, &user.Username, &user.PasswordHash)
		if err != nil {
			log.Println(err)
			continue
		}
		result = append(result, user)
	}

	return result
}

func (adapter *DatabaseAdapter) getMessagesFromGroup() []SavedMessage {
	result := []SavedMessage{}

	selectSql := sq.Select("messages.*, users.username").
		From("messages").
		Join("users on messages.user_id = users.id").
		Where("chat_id = ?", groupChatId)
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := SavedMessage{}
		err := rows.Scan(&msg.Id, &msg.Text, &msg.UserData.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.UserData.Username)
		if err != nil {
			log.Println(err)
			continue
		}
		result = append(result, msg)
	}

	return result
}

func (adapter *DatabaseAdapter) getMessagesFromPrivate(fromUserId int64, toUserId int64) []SavedMessage {
	result := []SavedMessage{}

	selectSql := sq.Select("messages.*, users.username").
		From("messages").
		Join("users on messages.user_id = users.id").
		Where(sq.Or{sq.Eq{"user_id": fromUserId, "chat_id": toUserId},
			sq.Eq{"user_id": toUserId, "chat_id": fromUserId}})
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := SavedMessage{}
		err := rows.Scan(&msg.Id, &msg.Text, &msg.UserData.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.UserData.Username)
		if err != nil {
			log.Println(err)
			continue
		}
		result = append(result, msg)
	}

	return result
}

func (adapter *DatabaseAdapter) getMessagesFromChat(userId int64, chatId int64) []SavedMessage {
	if chatId == 0 {
		return adapter.getMessagesFromGroup()
	}
	return adapter.getMessagesFromPrivate(userId, chatId)
}

func (adapter *DatabaseAdapter) getUserById(id int) (User, error) {
	user := User{}

	selectSql := sq.Select("*").From("users").Where("id = ?", id)
	row := selectSql.RunWith(adapter.DB).QueryRow()

	err := row.Scan(&user.Id, &user.Username, &user.PasswordHash)
	if err != nil {
		return User{}, errors.New("User with id = " + strconv.Itoa(id) +
			" does not exist. " + err.Error())
	}

	return user, nil
}

func (adapter *DatabaseAdapter) getUserByName(username string) (User, error) {
	user := User{}

	selectSql := sq.Select("*").From("users").Where("username = ?", username)
	row := selectSql.RunWith(adapter.DB).QueryRow()

	err := row.Scan(&user.Id, &user.Username, &user.PasswordHash)
	if err != nil {
		return User{}, errors.New("User with name = " + username +
			" does not exist. " + err.Error())
	}

	return user, nil
}

func (adapter *DatabaseAdapter) addNewUser(user *User) {
	insertSql := sq.Insert("users").Columns("username, password_hash").
		Values(user.Username, user.PasswordHash)
	result, err := insertSql.RunWith(adapter.DB).Exec()
	if err != nil {
		panic(err)
	}
	user.Id, err = result.LastInsertId()
	if err != nil {
		log.Println(err)
	}
}

func (adapter *DatabaseAdapter) addNewMessage(msg Message) SavedMessage {
	savedMessage := SavedMessage{UserData: msg.User.getPublicInfo(),
		ChatId: msg.ChatId, Text: msg.Text}
	savedMessage.CreatedOn = 0

	insertSql := sq.Insert("messages").Columns("chat_id, user_id, text, created_on").
		Values(msg.ChatId, msg.User.Id, msg.Text, savedMessage.CreatedOn)

	result, err := insertSql.RunWith(adapter.DB).Exec()
	if err != nil {
		panic(err)
	}
	savedMessage.Id, err = result.LastInsertId()
	if err != nil {
		log.Println(err)
	}
	return savedMessage
}

func (adapter *DatabaseAdapter) isUserExist(username string) bool {
	_, err := adapter.getUserByName(username)
	if err == nil {
		return true
	}
	return false
}

func (adapter *DatabaseAdapter) checkUserPassword(username string, passwordHash string) bool {
	user, err := adapter.getUserByName(username)
	if err == nil && user.PasswordHash == passwordHash {
		return true
	}
	return false
}
