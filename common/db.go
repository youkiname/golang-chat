// db
package common

import (
	"database/sql"
	"errors"
	"log"
	"strconv"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
)

type DatabaseAdapter struct {
	dbFileName string
	DB         *sql.DB
}

func (adapter *DatabaseAdapter) Close() {
	adapter.DB.Close()
}

func (adapter *DatabaseAdapter) ConnectSqlite(dbName string) {
	TABLES := []string{
		"users (id INTEGER PRIMARY KEY, username VARCHAR(64), password_hash VARCHAR(256));",

		`failed_login (id INTEGER PRIMARY KEY, 
		 user_id INTEGER NOT NULL, 
		 created_on INTEGER NOT NULL,
		 FOREIGN KEY (user_id) REFERENCES users(id));`,

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
	if IsError(err) {
		panic(err)
	}

	// Create initial tables
	for _, tableData := range TABLES {
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS " + tableData)
		if IsError(err) {
			panic(err)
		}
	}
	adapter.dbFileName = dbName
	adapter.DB = db
}

func (adapter *DatabaseAdapter) GetAllUsers() []User {
	db := adapter.DB
	result := []User{}

	usersSql := sq.Select("*").From("users")
	rows, err := usersSql.RunWith(db).Query()
	if IsError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		user := User{}
		err := rows.Scan(&user.Id, &user.Username, &user.PasswordHash)
		if IsError(err) {
			log.Println(err)
			continue
		}
		result = append(result, user)
	}

	return result
}

func (adapter *DatabaseAdapter) GetMessagesFromGroup() []SavedMessage {
	result := []SavedMessage{}

	selectSql := sq.Select("messages.*, users.username").
		From("messages").
		Join("users on messages.user_id = users.id").
		Where("chat_id = ?", GROUP_CHAT_ID)
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if IsError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := SavedMessage{}
		err := rows.Scan(&msg.Id, &msg.Text, &msg.UserData.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.UserData.Username)
		if IsError(err) {
			log.Println(err)
			continue
		}
		result = append(result, msg)
	}

	return result
}

func (adapter *DatabaseAdapter) GetMessagesFromPrivate(fromUserId int64, toUserId int64) []SavedMessage {
	result := []SavedMessage{}

	selectSql := sq.Select("messages.*, users.username").
		From("messages").
		Join("users on messages.user_id = users.id").
		Where(sq.Or{sq.Eq{"user_id": fromUserId, "chat_id": toUserId},
			sq.Eq{"user_id": toUserId, "chat_id": fromUserId}})
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if IsError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := SavedMessage{}
		err := rows.Scan(&msg.Id, &msg.Text, &msg.UserData.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.UserData.Username)
		if IsError(err) {
			log.Println(err)
			continue
		}
		result = append(result, msg)
	}

	return result
}

func (adapter *DatabaseAdapter) GetMessagesFromChat(userId int64, chatId int64) []SavedMessage {
	if chatId == 0 {
		return adapter.GetMessagesFromGroup()
	}
	return adapter.GetMessagesFromPrivate(userId, chatId)
}

func (adapter *DatabaseAdapter) GetUserById(id int) (User, error) {
	user := User{}

	selectSql := sq.Select("*").From("users").Where("id = ?", id)
	row := selectSql.RunWith(adapter.DB).QueryRow()

	err := row.Scan(&user.Id, &user.Username, &user.PasswordHash)
	if IsError(err) {
		return User{}, errors.New("User with id = " + strconv.Itoa(id) +
			" does not exist. " + err.Error())
	}

	return user, nil
}

func (adapter *DatabaseAdapter) GetUserByName(username string) (User, error) {
	user := User{}

	selectSql := sq.Select("*").From("users").Where("username = ?", username)
	row := selectSql.RunWith(adapter.DB).QueryRow()

	err := row.Scan(&user.Id, &user.Username, &user.PasswordHash)
	if IsError(err) {
		return User{}, errors.New("User with name = " + username +
			" does not exist. " + err.Error())
	}

	return user, nil
}

func (adapter *DatabaseAdapter) GetChannels(userId int64) []Channel {
	result := []Channel{}

	selectSql := sq.Select("saved_channels.chat_id, users.username").
		From("saved_channels").
		Join("users on saved_channels.chat_id = users.id").
		Where("user_id = ?", userId)
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if IsError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		channel := Channel{}
		err := rows.Scan(&channel.Id, &channel.Title)
		if IsError(err) {
			log.Println(err)
			continue
		}
		result = append(result, channel)
	}

	return result
}

func (adapter *DatabaseAdapter) GetLastFailedLoginDate(userId int64) int64 {
	query := sq.Select("created_on").From("failed_login").
		Where(sq.Eq{"user_id": userId}).OrderBy("created_on DESC").Limit(1)
	row := query.RunWith(adapter.DB).QueryRow()

	var result int64 = 0
	err := row.Scan(&result)
	if IsError(err) {
		return 0
	}

	return result
}

func (adapter *DatabaseAdapter) AddNewUser(user *User) {
	insertSql := sq.Insert("users").Columns("username, password_hash").
		Values(user.Username, user.PasswordHash)
	result, err := insertSql.RunWith(adapter.DB).Exec()
	if IsError(err) {
		panic(err)
	}
	user.Id, err = result.LastInsertId()
	if IsError(err) {
		log.Println(err)
	}
}

func (adapter *DatabaseAdapter) AddNewFailedLogin(userId int64) {
	insertSql := sq.Insert("failed_login").Columns("user_id, created_on").
		Values(userId, GetTimestampNow())
	_, err := insertSql.RunWith(adapter.DB).Exec()
	if IsError(err) {
		panic(err)
	}
}

func (adapter *DatabaseAdapter) AddNewMessage(msg Message) SavedMessage {
	savedMessage := SavedMessage{UserData: msg.User.GetPublicInfo(),
		ChatId: msg.ChatId, Text: msg.Text}
	savedMessage.CreatedOn = GetTimestampNow()

	insertSql := sq.Insert("messages").Columns("chat_id, user_id, text, created_on").
		Values(msg.ChatId, msg.User.Id, msg.Text, savedMessage.CreatedOn)

	result, err := insertSql.RunWith(adapter.DB).Exec()
	if IsError(err) {
		panic(err)
	}
	savedMessage.Id, err = result.LastInsertId()
	if IsError(err) {
		log.Println(err)
	}
	return savedMessage
}

func (adapter *DatabaseAdapter) AddNewChannel(userId int64, chatId int64) {
	// this function adds new user(user_id) to user(chat_id) private relationship
	insertSql := sq.Insert("saved_channels").Columns("user_id, chat_id").
		Values(userId, chatId)

	_, err := insertSql.RunWith(adapter.DB).Exec()
	if IsError(err) {
		panic(err)
	}
}

func (adapter *DatabaseAdapter) IsUserExist(username string) bool {
	_, err := adapter.GetUserByName(username)
	if !IsError(err) {
		return true
	}
	return false
}

func (adapter *DatabaseAdapter) IsChannelExist(userId, chatId int64) bool {
	selectSql := sq.Select("*").From("saved_channels").
		Where("user_id = ? AND chat_id = ?", userId, chatId)
	row := selectSql.RunWith(adapter.DB).QueryRow()
	// next 3 variables are used only to enter Scan arguments
	var tempId int
	var tempUserId int64
	var tempChatId int64
	err := row.Scan(&tempId, &tempUserId, &tempChatId)
	if IsError(err) { // not found
		return false
	}
	return true
}

func (adapter *DatabaseAdapter) CheckUserPassword(username, passwordHash string) bool {
	user, err := adapter.GetUserByName(username)
	if !IsError(err) && user.PasswordHash == passwordHash {
		return true
	}
	return false
}

func (adapter *DatabaseAdapter) CountFailedLogin(userId int64) int {
	var count int
	row := adapter.DB.QueryRow("SELECT COUNT(user_id = ?) FROM failed_login", userId)
	err := row.Scan(&count)
	if IsError(err) {
		panic(err)
	}
	return count
}

func (adapter *DatabaseAdapter) ClearFailedLogin(userId int64) {
	query := sq.Delete("failed_login").Where(sq.Eq{"user_id": userId})
	_, err := query.RunWith(adapter.DB).Exec()
	if IsError(err) {
		panic(err)
	}
}
