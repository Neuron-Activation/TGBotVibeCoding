package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	tele "gopkg.in/telebot.v3"
)

// --- Structures ---

// UserData хранит информацию, собранную о пользователе
type UserData struct {
	Name     string
	Age      string
	Bio      string
	Children string
}

// ConversationState хранит текущий шаг диалога
type ConversationState int

const (
	StateChoosing ConversationState = iota
	StateTypingChoice
	StateTypingReply
)

// Store объединяет состояние диалога и данные пользователя
type Store struct {
	State map[int64]ConversationState `json:"states"`
	Data  map[int64]*UserData         `json:"data"`
	Ctx   map[int64]string            `json:"context"` // Хранит, какую категорию сейчас редактирует юзер
}

// PersistenceHandler управляет сохранением/загрузкой
type PersistenceHandler struct {
	FilePath string
	Mu       sync.RWMutex
	Store    Store
}

// --- Persistence Logic ---

func NewPersistence(path string) *PersistenceHandler {
	p := &PersistenceHandler{
		FilePath: path,
		Store: Store{
			State: make(map[int64]ConversationState),
			Data:  make(map[int64]*UserData),
			Ctx:   make(map[int64]string),
		},
	}
	p.Load()
	return p
}

func (p *PersistenceHandler) Save() error {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	data, err := json.MarshalIndent(p.Store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.FilePath, data, 0644)
}

func (p *PersistenceHandler) Load() {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	file, err := os.ReadFile(p.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Файл данных не найден, создаем новый при сохранении.")
			return
		}
		log.Printf("Ошибка чтения файла: %v", err)
		return
	}
	json.Unmarshal(file, &p.Store)
}

func (p *PersistenceHandler) GetState(userID int64) ConversationState {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	return p.Store.State[userID]
}

func (p *PersistenceHandler) SetState(userID int64, state ConversationState) {
	p.Mu.Lock()
	p.Store.State[userID] = state
	p.Mu.Unlock()
	p.Save()
}

func (p *PersistenceHandler) UpdateData(userID int64, key, value string) {
	p.Mu.Lock()

	if _, ok := p.Store.Data[userID]; !ok {
		p.Store.Data[userID] = &UserData{}
	}

	switch key {
	case "Name":
		p.Store.Data[userID].Name = value
	case "Age":
		p.Store.Data[userID].Age = value
	case "Bio":
		p.Store.Data[userID].Bio = value
	case "Children":
		p.Store.Data[userID].Children = value
	}
    
    // Снимаем Lock, чтобы p.Save() смог взять свой Lock.
    p.Mu.Unlock() 
	
	p.Save() 
}

func (p *PersistenceHandler) SetContext(userID int64, val string) {
	p.Mu.Lock()
	p.Store.Ctx[userID] = val
	p.Mu.Unlock()
	p.Save()
}

func (p *PersistenceHandler) GetContext(userID int64) string {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	return p.Store.Ctx[userID]
}

func (p *PersistenceHandler) GetDataString(userID int64) string {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	if d, ok := p.Store.Data[userID]; ok {
		return fmt.Sprintf("Name: %s\nAge: %s\nBio: %s\nChildren: %s", d.Name, d.Age, d.Bio, d.Children)
	}
	return "Нет данных."
}

// --- Bot Logic ---

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN не задан")
	}

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}

	// Инициализация хранилища
	storage := NewPersistence("data.json")

	// Меню
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}
	btnName := menu.Text("Name")
	btnAge := menu.Text("Age")
	btnBio := menu.Text("Bio")
	btnChildren := menu.Text("Children")
	btnDone := menu.Text("Done")

	menu.Reply(
		menu.Row(btnName, btnAge),
		menu.Row(btnBio, btnChildren),
		menu.Row(btnDone),
	)

	// Handlers

	// /start
	b.Handle("/start", func(c tele.Context) error {
		storage.SetState(c.Sender().ID, StateChoosing)
		return c.Send("Привет! Я помогу сохранить факты о тебе. Что ты хочешь рассказать?", menu)
	})

	// /show_data
	b.Handle("/show_data", func(c tele.Context) error {
		return c.Send(fmt.Sprintf("Вот что я знаю:\n%s", storage.GetDataString(c.Sender().ID)))
	})

	// Обработка кнопок выбора (Regular Expression для кнопок)
	b.Handle(tele.OnText, func(c tele.Context) error {
		userID := c.Sender().ID
		text := c.Text()
		currentState := storage.GetState(userID)

		// 1. Если состояние CHOOSING (выбор категории)
		if currentState == StateChoosing {
			switch text {
			case "Name", "Age", "Bio", "Children":
				storage.SetContext(userID, text)
				storage.SetState(userID, StateTypingReply)
				return c.Send(fmt.Sprintf("Хорошо, введи свои данные для %s:", text))
			case "Done":
				return c.Send(fmt.Sprintf("Я запомнил:\n%s\nДо встречи!", storage.GetDataString(userID)), tele.RemoveKeyboard)
			default:
				return c.Send("Пожалуйста, выбери кнопку из меню.", menu)
			}
		}

		// 2. Если состояние TYPING_REPLY (ввод данных)
		if currentState == StateTypingReply {
			category := storage.GetContext(userID)
			storage.UpdateData(userID, category, text)
			storage.SetState(userID, StateChoosing)
			return c.Send(fmt.Sprintf("Записал %s! Что еще?", category), menu)
		}

		// Если состояние неизвестно или сброшено
		return c.Send("Напиши /start, чтобы начать диалог.")
	})

	log.Println("Бот запущен...")
	b.Start()
}