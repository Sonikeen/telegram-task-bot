package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Task struct {
	Text string
}

type UserState struct {
	CurrentAction string
	TempData      string // Временные данные для хранения введенного текста задач
}

var (
	userTasks  = make(map[int64][]Task)     // Словарь задач для каждого пользователя
	userStates = make(map[int64]*UserState) // Словарь состояния пользователей
	mu         sync.Mutex                   // Мьютекс для защиты от параллельного доступа
)

func showMainMenu(chatID int64, bot *tgbotapi.BotAPI) {
	menu := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Добавить задачи"),
			tgbotapi.NewKeyboardButton("Список задач"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Очистить задачи"),
			tgbotapi.NewKeyboardButton("Удалить задачу"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Редактировать задачу"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = menu
	bot.Send(msg)
}

func addTasks(chatID int64, bot *tgbotapi.BotAPI, message string) {
	if message != "" {
		tasks := strings.Split(message, "\n") // Разделяем задачи по строкам
		for _, task := range tasks {
			task = strings.TrimSpace(task) // Убираем лишние пробелы
			if task != "" {
				mu.Lock()
				userTasks[chatID] = append(userTasks[chatID], Task{Text: task}) // Добавляем задачу
				mu.Unlock()
			}
		}
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Добавлено задач: %d", len(tasks)))
		bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(chatID, "Сообщение пустое. Попробуйте снова.")
		bot.Send(msg)
	}
	showMainMenu(chatID, bot)
}

func listTasks(chatID int64, bot *tgbotapi.BotAPI) {
	mu.Lock()
	tasks, exists := userTasks[chatID]
	mu.Unlock()

	if !exists || len(tasks) == 0 { // Если задач нет
		msg := tgbotapi.NewMessage(chatID, "Список задач пуст.")
		bot.Send(msg)
	} else {
		var taskListText string
		for i, task := range tasks {
			taskListText += fmt.Sprintf("%d. %s\n", i+1, task.Text)
		}
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Список задач:\n%s", taskListText))
		bot.Send(msg)
	}
}

func clearTasks(chatID int64, bot *tgbotapi.BotAPI) {
	mu.Lock()
	delete(userTasks, chatID)
	mu.Unlock()
	msg := tgbotapi.NewMessage(chatID, "Все задачи были удалены.")
	bot.Send(msg)
	showMainMenu(chatID, bot)
}

func deleteTask(chatID int64, bot *tgbotapi.BotAPI) {
	mu.Lock()
	tasks, exists := userTasks[chatID]
	mu.Unlock()
	if !exists || len(tasks) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Список задач пуст. Сначала добавьте задачи.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Введите номер задачи, которую хотите удалить:")
	bot.Send(msg)

	// Ожидаем номер задачи от пользователя
	mu.Lock()
	userStates[chatID] = &UserState{CurrentAction: "deleting"}
	mu.Unlock()
}

func editTask(chatID int64, bot *tgbotapi.BotAPI) {
	mu.Lock()
	tasks, exists := userTasks[chatID]
	mu.Unlock()
	if !exists || len(tasks) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Список задач пуст. Сначала добавьте задачи.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Введите номер задачи, которую хотите отредактировать:")
	bot.Send(msg)

	// Ожидаем номер задачи от пользователя
	mu.Lock()
	userStates[chatID] = &UserState{CurrentAction: "editing"}
	mu.Unlock()
}

func handleUserInput(update tgbotapi.Update, bot *tgbotapi.BotAPI) {
	chatID := update.Message.Chat.ID
	mu.Lock()
	userState, stateExists := userStates[chatID]
	mu.Unlock()

	if update.Message != nil && stateExists {
		switch userState.CurrentAction {
		case "deleting":
			// Проверяем, что введено целое число
			taskNumber, err := strconv.Atoi(update.Message.Text)
			if err != nil || taskNumber < 1 {
				msg := tgbotapi.NewMessage(chatID, "Неверный номер задачи. Попробуйте снова.")
				bot.Send(msg)
				return
			}

			mu.Lock()
			tasks := userTasks[chatID]
			if taskNumber > len(tasks) {
				msg := tgbotapi.NewMessage(chatID, "Задача с таким номером не существует.")
				bot.Send(msg)
				mu.Unlock()
				return
			}
			userTasks[chatID] = append(tasks[:taskNumber-1], tasks[taskNumber:]...)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Задача #%d удалена.", taskNumber))
			bot.Send(msg)
			mu.Unlock()
			// Сброс состояния после выполнения действия
			mu.Lock()
			delete(userStates, chatID)
			mu.Unlock()
			showMainMenu(chatID, bot)

		case "editing":
			// Проверяем, что введено целое число
			taskNumber, err := strconv.Atoi(update.Message.Text)
			if err != nil || taskNumber < 1 {
				msg := tgbotapi.NewMessage(chatID, "Неверный номер задачи. Попробуйте снова.")
				bot.Send(msg)
				return
			}

			mu.Lock()
			tasks := userTasks[chatID]
			if taskNumber > len(tasks) {
				msg := tgbotapi.NewMessage(chatID, "Задача с таким номером не существует.")
				bot.Send(msg)
				mu.Unlock()
				return
			}
			userState.TempData = strconv.Itoa(taskNumber)
			mu.Unlock()

			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Введите новый текст для задачи #%d:", taskNumber))
			bot.Send(msg)

			mu.Lock()
			userStates[chatID].CurrentAction = "editing_text"
			mu.Unlock()

		case "editing_text":
			// Обрабатываем новый текст задачи
			newText := update.Message.Text
			if newText != "" {
				mu.Lock()
				tasks := userTasks[chatID]
				taskNumberStr := userState.TempData
				taskNumber, err := strconv.Atoi(taskNumberStr)
				if err != nil {
					msg := tgbotapi.NewMessage(chatID, "Ошибка при обработке номера задачи. Попробуйте снова.")
					bot.Send(msg)
					mu.Unlock()
					return
				}

				// Обновляем задачу
				tasks[taskNumber-1].Text = newText
				userTasks[chatID] = tasks
				mu.Unlock()

				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Задача отредактирована: %s", newText))
				bot.Send(msg)

				// Показываем обновленный список
				listTasks(chatID, bot)

				// Сброс состояния
				mu.Lock()
				delete(userStates, chatID)
				mu.Unlock()
			} else {
				msg := tgbotapi.NewMessage(chatID, "Текст задачи не может быть пустым. Попробуйте снова.")
				bot.Send(msg)
			}

		case "adding_task":
			// После нажатия кнопки "Добавить задачу" ожидаем ввод задач
			addTasks(chatID, bot, update.Message.Text)
			// Сброс состояния
			mu.Lock()
			delete(userStates, chatID)
			mu.Unlock()
		}
	}
}

func main() {
	bot, err := tgbotapi.NewBotAPI("7791341852:AAGAdd9ZDOtZbiegHSzqXIalAJmdKOxfuPg") // Замените на ваш ключ
	if err != nil {
		fmt.Println("Ошибка подключения к боту:", err)
		return
	}

	bot.Debug = true
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			switch update.Message.Text {
			case "/start", "Главное меню":
				showMainMenu(update.Message.Chat.ID, bot)
			case "Добавить задачи":
				// Отправляем сообщение и меняем состояние
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Введите задачи, каждая с новой строки:")
				bot.Send(msg)
				mu.Lock()
				userStates[update.Message.Chat.ID] = &UserState{CurrentAction: "adding_task"}
				mu.Unlock()
			case "Список задач":
				listTasks(update.Message.Chat.ID, bot)
			case "Очистить задачи":
				clearTasks(update.Message.Chat.ID, bot)
			case "Удалить задачу":
				deleteTask(update.Message.Chat.ID, bot)
			case "Редактировать задачу":
				editTask(update.Message.Chat.ID, bot)
			default:
				handleUserInput(update, bot)
			}
		}
	}
}
