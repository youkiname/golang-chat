// db
package db

import (
	"database/sql"
	"errors"
	"log"
	"strconv"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
	"github.com/satori/go.uuid"

	"chat/encrypt"
	"chat/models"
	"chat/utils"
)

type DatabaseAdapter struct {
	dbFileName string
	DB         *sql.DB
	CommonKey  uuid.UUID
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
	if utils.IsError(err) {
		panic(err)
	}

	// Create initial tables
	for _, tableData := range TABLES {
		_, err := db.Exec("CREATE TABLE IF NOT EXISTS " + tableData)
		if utils.IsError(err) {
			panic(err)
		}
	}
	adapter.dbFileName = dbName
	adapter.DB = db
	adapter.CommonKey = uuid.FromBytesOrNil([]byte(utils.COMMON_SECRET_KEY))
}

func (adapter *DatabaseAdapter) GetMessagesFromGroup() []models.SavedMessage {
	// returns all messages from group channel
	result := []models.SavedMessage{}
	selectSql := sq.Select("messages.*, users.username").
		From("messages").
		Join("users on messages.user_id = users.id").
		Where("chat_id = ?", utils.GROUP_CHAT_ID)
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if utils.IsError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := models.SavedMessage{}
		encryptedText := ""
		err := rows.Scan(&msg.Id, &encryptedText, &msg.User.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.User.Username)
		if utils.IsError(err) {
			log.Println(err)
			continue
		}
		decryptedText, _ := encrypt.DecryptText(adapter.CommonKey.Bytes(), encryptedText)
		msg.Text = decryptedText
		result = append(result, msg)
	}

	return result
}

func (adapter *DatabaseAdapter) GetMessagesFromPrivate(fromUserId int64, toUserId int64) []models.SavedMessage {
	result := []models.SavedMessage{}

	selectSql := sq.Select("messages.*, users.username").
		From("messages").
		Join("users on messages.user_id = users.id").
		Where(sq.Or{sq.Eq{"user_id": fromUserId, "chat_id": toUserId},
			sq.Eq{"user_id": toUserId, "chat_id": fromUserId}})
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if utils.IsError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		msg := models.SavedMessage{}
		encryptedText := ""
		err := rows.Scan(&msg.Id, &encryptedText, &msg.User.Id, &msg.ChatId,
			&msg.CreatedOn, &msg.User.Username)
		if utils.IsError(err) {
			log.Println(err)
			continue
		}
		decryptedText, _ := encrypt.DecryptText(adapter.CommonKey.Bytes(), encryptedText)
		msg.Text = decryptedText
		result = append(result, msg)
	}

	return result
}

func (adapter *DatabaseAdapter) GetMessagesFromChat(userId int64, chatId int64) []models.SavedMessage {
	if chatId == 0 {
		return adapter.GetMessagesFromGroup()
	}
	return adapter.GetMessagesFromPrivate(userId, chatId)
}

func (adapter *DatabaseAdapter) GetUserById(id int) (models.User, error) {
	user := models.User{}

	selectSql := sq.Select("id, username").From("users").Where("id = ?", id)
	row := selectSql.RunWith(adapter.DB).QueryRow()

	err := row.Scan(&user.Id, &user.Username)
	if utils.IsError(err) {
		return models.User{}, errors.New("User with id = " + strconv.Itoa(id) +
			" does not exist. " + err.Error())
	}

	return user, nil
}

func (adapter *DatabaseAdapter) GetUserByName(username string) (models.User, error) {
	user := models.User{}

	selectSql := sq.Select("id, username").From("users").Where("username = ?", username)
	row := selectSql.RunWith(adapter.DB).QueryRow()

	err := row.Scan(&user.Id, &user.Username)
	if utils.IsError(err) {
		return models.User{}, errors.New("User with name = " + username +
			" does not exist. " + err.Error())
	}

	return user, nil
}

func (adapter *DatabaseAdapter) GetChannels(userId int64) []models.Channel {
	result := []models.Channel{}

	selectSql := sq.Select("saved_channels.chat_id, users.username").
		From("saved_channels").
		Join("users on saved_channels.chat_id = users.id").
		Where("user_id = ?", userId)
	rows, err := selectSql.RunWith(adapter.DB).Query()
	if utils.IsError(err) {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		channel := models.Channel{}
		err := rows.Scan(&channel.Id, &channel.Title)
		if utils.IsError(err) {
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
	if utils.IsError(err) {
		return 0
	}

	return result
}

func (adapter *DatabaseAdapter) AddNewUser(user *models.User, passwordHash string) {
	insertSql := sq.Insert("users").Columns("username, password_hash").
		Values(user.Username, passwordHash)
	result, err := insertSql.RunWith(adapter.DB).Exec()
	if utils.IsError(err) {
		panic(err)
	}
	user.Id, err = result.LastInsertId()
	if utils.IsError(err) {
		log.Println(err)
	}
}

func (adapter *DatabaseAdapter) AddNewFailedLogin(userId int64) {
	insertSql := sq.Insert("failed_login").Columns("user_id, created_on").
		Values(userId, utils.GetTimestampNow())
	_, err := insertSql.RunWith(adapter.DB).Exec()
	if utils.IsError(err) {
		panic(err)
	}
}

func (adapter *DatabaseAdapter) AddNewMessage(msg models.Message) models.SavedMessage {
	savedMessage := models.SavedMessage{Message: msg}
	savedMessage.CreatedOn = utils.GetTimestampNow()

	encryptedText, err := encrypt.EncryptText(adapter.CommonKey.Bytes(), msg.Text)

	insertSql := sq.Insert("messages").Columns("chat_id, user_id, text, created_on").
		Values(msg.ChatId, msg.User.Id, encryptedText, savedMessage.CreatedOn)

	result, err := insertSql.RunWith(adapter.DB).Exec()
	if utils.IsError(err) {
		panic(err)
	}
	savedMessage.Id, err = result.LastInsertId()
	if utils.IsError(err) {
		log.Println(err)
	}
	return savedMessage
}

func (adapter *DatabaseAdapter) AddNewChannel(userId int64, chatId int64) {
	// this function adds new user(user_id) to user(chat_id) private relationship
	insertSql := sq.Insert("saved_channels").Columns("user_id, chat_id").
		Values(userId, chatId)

	_, err := insertSql.RunWith(adapter.DB).Exec()
	if utils.IsError(err) {
		panic(err)
	}
}

func (adapter *DatabaseAdapter) IsUserExist(username string) bool {
	_, err := adapter.GetUserByName(username)
	if !utils.IsError(err) {
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
	if utils.IsError(err) { // not found
		return false
	}
	return true
}

func (adapter *DatabaseAdapter) CheckUserPassword(username, passwordHash string) bool {
	selectSql := sq.Select("password_hash").From("users").Where("username = ?", username)
	row := selectSql.RunWith(adapter.DB).QueryRow()

	savedPasswordHash := ""
	err := row.Scan(&savedPasswordHash)
	if !utils.IsError(err) && savedPasswordHash == passwordHash {
		return true
	}
	return false
}

func (adapter *DatabaseAdapter) CountFailedLogin(userId int64) int {
	var count int
	row := adapter.DB.QueryRow("SELECT COUNT(user_id = ?) FROM failed_login", userId)
	err := row.Scan(&count)
	if utils.IsError(err) {
		panic(err)
	}
	return count
}

func (adapter *DatabaseAdapter) ClearFailedLogin(userId int64) {
	query := sq.Delete("failed_login").Where(sq.Eq{"user_id": userId})
	_, err := query.RunWith(adapter.DB).Exec()
	if utils.IsError(err) {
		panic(err)
	}
}
