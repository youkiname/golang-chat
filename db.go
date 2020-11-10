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
		FOREIGN KEY (user_id) REFERENCES users(id));`,

		`saved_channels
		(id INTEGER PRIMARY KEY,
		user_id INTEGER NOT NULL,
		chat_id INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (chat_id) REFERENCES users(id));`}

	db, err := sql.Open("sqlite3", dbName)
	if isError(err) {
		panic(err)
	}

	// Create initial tables
	for _, tableData := range TABLES {
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS " + tableData)
		if isError(err) {
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
	if isError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		user := User{}
		err := rows.Scan(&user.Id, &user.Username, &user.PasswordHash)
		if isError(err) {
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
	if isError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := SavedMessage{}
		err := rows.Scan(&msg.Id, &msg.Text, &msg.UserData.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.UserData.Username)
		if isError(err) {
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
	if isError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := SavedMessage{}
		err := rows.Scan(&msg.Id, &msg.Text, &msg.UserData.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.UserData.Username)
		if isError(err) {
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
	if isError(err) {
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
	if isError(err) {
		return User{}, errors.New("User with name = " + username +
			" does not exist. " + err.Error())
	}

	return user, nil
}

func (adapter *DatabaseAdapter) getChannels(userId int64) []Channel {
	result := []Channel{}

	selectSql := sq.Select("saved_channels.chat_id, users.username").
		From("saved_channels").
		Join("users on saved_channels.chat_id = users.id").
		Where("user_id = ?", userId)
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if isError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		channel := Channel{}
		err := rows.Scan(&channel.Id, &channel.Title)
		if isError(err) {
			log.Println(err)
			continue
		}
		result = append(result, channel)
	}

	return result
}

func (adapter *DatabaseAdapter) addNewUser(user *User) {
	insertSql := sq.Insert("users").Columns("username, password_hash").
		Values(user.Username, user.PasswordHash)
	result, err := insertSql.RunWith(adapter.DB).Exec()
	if isError(err) {
		panic(err)
	}
	user.Id, err = result.LastInsertId()
	if isError(err) {
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
	if isError(err) {
		panic(err)
	}
	savedMessage.Id, err = result.LastInsertId()
	if isError(err) {
		log.Println(err)
	}
	return savedMessage
}

func (adapter *DatabaseAdapter) addNewChannel(userId int64, chatId int64) {
	// this function adds new user(user_id) to user(chat_id) private relationship
	insertSql := sq.Insert("saved_channels").Columns("user_id, chat_id").
		Values(userId, chatId)

	_, err := insertSql.RunWith(adapter.DB).Exec()
	if isError(err) {
		panic(err)
	}
}

func (adapter *DatabaseAdapter) isUserExist(username string) bool {
	_, err := adapter.getUserByName(username)
	if !isError(err) {
		return true
	}
	return false
}

func (adapter *DatabaseAdapter) isChannelExist(userId, chatId int64) bool {
	selectSql := sq.Select("*").From("saved_channels").
		Where("user_id = ? AND chat_id = ?", userId, chatId)
	row := selectSql.RunWith(adapter.DB).QueryRow()
	// next 3 variables are used only to enter Scan arguments
	var tempId int
	var tempUserId int64
	var tempChatId int64
	err := row.Scan(&tempId, &tempUserId, &tempChatId)
	if isError(err) { // not found
		return false
	}
	return true
}

func (adapter *DatabaseAdapter) checkUserPassword(username, passwordHash string) bool {
	user, err := adapter.getUserByName(username)
	if !isError(err) && user.PasswordHash == passwordHash {
		return true
	}
	return false
}
