package bot

import (
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"tournament-bot/internal/db"
	"tournament-bot/internal/notifications"
	"tournament-bot/internal/services"
)

const (
	StateAwaitingScore1 = iota
	StateAwaitingScore2
	StateAwaitingOvertimeScore
	StateAwaitingPenaltiesScore
)

type TeamSelectionState struct {
	TournamentID      int
	Team1             string
	Team2             string
	Score1            int
	Score2            int
	ConversationState int
	AwaitingScore     bool
}

var teamSelectionStates = make(map[int64]*TeamSelectionState)

func HandleMessage(message *tgbotapi.Message) {
	if message.IsCommand() {
		switch message.Command() {
		case "add_participant":
			addParticipantHandler(message)
		case "create_tournament":
			createTournamentHandler(message)
		case "end_tournament":
			endTournament(message)
		case "delete_tournament":
			HandleDeleteTournament(message)
		case "add_team_category":
			addTeamCategoryHandler(message)
		case "add_match":
			addMatchHandler(message)
		case "addadmin":
			handleAddAdminCommand(message)
		case "removeadmin":
			handleRemoveAdminCommand(message)

		case "tournament_info":
			tournamentInfoHandler(message)
		case "deletelastmatch":
			deleteLastMatchHandler(message)

		case "start_playoff":
			startPlayoffHandler(message)
		case "cancel":
			// Проверка наличия активного состояния выбора команд для пользователя
			_, ok := teamSelectionStates[message.From.ID]
			if ok {
				// Отправка сообщения о прерывании процесса
				msg := tgbotapi.NewMessage(message.Chat.ID, "Текущий процесс добавления результата матча был прерван. Вы можете начать новый процесс с помощью команды /add_match.")
				bot.Send(msg)

				// Сброс состояния выбора команд для пользователя
				delete(teamSelectionStates, message.From.ID)
			}
		}

	} else {
		// Проверяем, есть ли активное состояние выбора команд для пользователя
		state, ok := teamSelectionStates[message.From.ID]
		if ok {
			switch state.ConversationState {
			case StateAwaitingScore1, StateAwaitingScore2:
				// Проверяем, является ли сообщение числом (счетом)
				if _, err := strconv.Atoi(message.Text); err == nil {
					handleScoreInput(message)
				} else {
					// Отправляем сообщение о некорректном вводе
					msg := tgbotapi.NewMessage(message.Chat.ID, "Пожалуйста, введите корректный счет (целое число).")
					bot.Send(msg)
				}
			case StateAwaitingOvertimeScore, StateAwaitingPenaltiesScore:
				// Проверяем, является ли сообщение счетом в формате "команда1:команда2"
				if strings.Contains(message.Text, ":") {
					handleScoreInput(message)
				} else {
					// Отправляем сообщение о некорректном вводе
					msg := tgbotapi.NewMessage(message.Chat.ID, "Пожалуйста, введите счет в формате 'команда1:команда2'.")
					bot.Send(msg)
				}
			}
		}
	}
}

func addParticipantHandler(message *tgbotapi.Message) {
	// Получаем имя и фамилию участника из аргументов команды
	participantName := strings.TrimSpace(message.CommandArguments())
	if participantName == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Please provide a participant name and surname."))
		return
	}

	// Проверяем, что имя и фамилия состоят только из букв и пробелов
	if !isValidName(participantName) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid participant name. Please provide a valid name and surname."))
		return
	}

	// Проверяем, что участник еще не был добавлен
	exists, err := db.ParticipantExists(participantName)
	if err != nil {
		log.Printf("Error checking participant existence: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while checking participant existence."))
		return
	}
	if exists {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Participant %s already exists.", participantName)))
		return
	}

	// Добавляем участника в базу данных
	err = db.AddParticipant(participantName)
	if err != nil {
		log.Printf("Error adding participant: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while adding the participant."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Participant %s has been added.", participantName)))
}

func isValidName(name string) bool {
	// Проверяем, что имя и фамилия состоят только из букв и пробелов
	return regexp.MustCompile(`^[a-zA-Zа-яА-Я\s]+$`).MatchString(name)
}

func createTournamentHandler(message *tgbotapi.Message) {
	// Проверяем, является ли пользователь администратором
	isAdmin, err := db.IsAdmin(message.From.ID)
	if err != nil {
		log.Printf("Error checking admin status: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "An error occurred while checking your admin status.")
		bot.Send(msg)
		return
	}
	if !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "You don't have permission to create tournaments.")
		bot.Send(msg)
		return
	}
	// Проверка наличия активного турнира
	activeTournament, err := services.GetActiveTournament()
	if err != nil {
		log.Printf("Error getting active tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while checking for active tournament."))
		return
	}

	if activeTournament != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "There is already an active tournament. Please wait for it to finish."))
		return
	}

	// Создание нового турнира
	tournament, err := services.CreateTournament()
	if err != nil {
		log.Printf("Error creating tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while creating the tournament."))
		return
	}

	// Отправка сообщения с кнопками для добавления участников
	msg := tgbotapi.NewMessage(message.Chat.ID, "A new tournament has been created. Add participants:")
	msg.ReplyMarkup, _ = getParticipantsKeyboard(tournament.ID)
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}
}

func endTournamentHandler(message *tgbotapi.Message) {
	// Получение активного турнира
	activeTournament, err := services.GetActiveTournament()
	if err != nil {
		log.Printf("Error getting active tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while checking for active tournament."))
		return
	}

	if activeTournament == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "There is no active tournament to end."))
		return
	}

	// Завершение активного турнира
	err = services.EndTournament(activeTournament.ID)
	if err != nil {
		log.Printf("Error ending tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while ending the tournament."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "The active tournament has been ended."))
}

func getParticipantsKeyboard(tournamentID int) (tgbotapi.InlineKeyboardMarkup, error) {
	// Получаем список всех участников из базы данных
	participants, err := db.GetAllParticipants()
	if err != nil {
		log.Printf("Error getting participants: %v", err)
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, participant := range participants {
		var label string
		tournament, err := services.GetTournament(tournamentID)
		if err == nil && tournament.HasParticipant(participant) {
			label = "✅ " + participant
		} else {
			label = participant
		}
		callbackData := fmt.Sprintf("toggle_participant_%d_%s", tournamentID, participant)
		button := tgbotapi.NewInlineKeyboardButtonData(label, callbackData)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{button})
	}

	selectCategoryCallbackData := fmt.Sprintf("select_category_%d", tournamentID)
	selectCategoryButton := tgbotapi.NewInlineKeyboardButtonData("Select Category", selectCategoryCallbackData)
	rows = append(rows, []tgbotapi.InlineKeyboardButton{selectCategoryButton})

	return tgbotapi.NewInlineKeyboardMarkup(rows...), nil
}

func endTournament(message *tgbotapi.Message) {
	// Получаем идентификатор турнира из аргументов команды
	tournamentID, err := strconv.Atoi(message.CommandArguments())
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Invalid tournament ID"))
		return
	}

	// Завершаем турнир с указанным идентификатором
	err = services.EndTournament(tournamentID)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Error ending tournament: "+err.Error()))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Tournament ended"))
}

func СallbackHandler(callback *tgbotapi.CallbackQuery) {
	if strings.HasPrefix(callback.Data, "toggle_participant_") {
		parts := strings.Split(callback.Data, "_")
		tournamentID, err := strconv.Atoi(parts[2])
		if err != nil {
			log.Printf("Error converting tournament ID: %v", err)
			return
		}
		participantName := parts[3]

		tournament, err := services.GetTournament(tournamentID)
		if err != nil {
			log.Printf("Error getting tournament: %v", err)
			return
		}

		if tournament.IsActive {
			// Турнир уже начат, отправляем сообщение об ошибке
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "The tournament has already started. You cannot add or remove participants anymore.")
			bot.Send(msg)
			return
		}

		err = services.ToggleParticipant(tournamentID, participantName)
		if err != nil {
			log.Printf("Error toggling participant: %v", err)
			return
		}

		// Обновляем клавиатуру с участниками
		keyboard, err := getParticipantsKeyboard(tournamentID)
		if err != nil {
			log.Printf("Error getting participants keyboard: %v", err)
			return
		}
		msg := tgbotapi.NewEditMessageReplyMarkup(callback.Message.Chat.ID, callback.Message.MessageID, keyboard)
		_, err = bot.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		// Отвечаем на callback, чтобы убрать "часики" на кнопке
		bot.Request(tgbotapi.NewCallback(callback.ID, ""))
	} else if strings.HasPrefix(callback.Data, "delete_tournament_") {
		tournamentID, _ := strconv.Atoi(strings.TrimPrefix(callback.Data, "delete_tournament_"))
		err := services.DeleteTournament(tournamentID)
		if err != nil {
			log.Printf("Error deleting tournament: %v", err)
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Failed to delete the tournament.")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Tournament deleted successfully.")
			bot.Send(msg)
		}
	} else if strings.HasPrefix(callback.Data, "select_category_") {
		// Получаем идентификатор турнира из callback.Data
		tournamentID, err := strconv.Atoi(strings.TrimPrefix(callback.Data, "select_category_"))
		if err != nil {
			log.Printf("Error converting tournament ID: %v", err)
			return
		}

		tournament, err := services.GetTournament(tournamentID)
		if err != nil {
			log.Printf("Error getting tournament: %v", err)
			return
		}

		if tournament.IsActive {
			// Турнир уже начат, отправляем сообщение об ошибке
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "The tournament has already started. You cannot select categories anymore.")
			bot.Send(msg)
			return
		}

		// Отправляем сообщение с клавиатурой выбора категории команд
		keyboard, err := getTeamCategoriesKeyboard(tournamentID)
		if err != nil {
			log.Printf("Error getting team categories keyboard: %v", err)
			return
		}
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Select the team category for the tournament:")
		msg.ReplyMarkup = keyboard
		_, err = bot.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		// Отвечаем на callback, чтобы убрать "часики" на кнопке
		bot.Request(tgbotapi.NewCallback(callback.ID, ""))
	} else if strings.HasPrefix(callback.Data, "category_selected_") {
		parts := strings.Split(callback.Data, "_")
		tournamentID, err := strconv.Atoi(parts[2])
		if err != nil {
			log.Printf("Error converting tournament ID: %v", err)
			return
		}
		categoryName := parts[3]

		tournament, err := services.GetTournament(tournamentID)
		if err != nil {
			log.Printf("Error getting tournament: %v", err)
			return
		}

		if tournament.IsActive {
			// Турнир уже начат, отправляем сообщение об ошибке
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "The tournament has already started. You cannot change the category anymore.")
			bot.Send(msg)
			return
		}

		err = services.SetTournamentTeamCategory(tournamentID, categoryName)
		if err != nil {
			log.Printf("Error setting tournament team category: %v", err)
			return
		}

		// Выполняем жеребьевку команд
		drawResult, err := services.PerformTeamDraw(tournamentID)
		if err != nil {
			log.Printf("Error performing team draw: %v", err)
			return
		}

		// Запускаем турнир
		updatedTournament, err := services.StartTournament(tournamentID)
		if err != nil {
			log.Printf("Error starting tournament: %v", err)
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, err.Error())
			bot.Send(msg)
			return
		}

		// Отправляем сообщение с результатом жеребьевки и информацией о начале турнира
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Team draw result:\n"+drawResult+"\nThe tournament has started!")
		_, err = bot.Send(msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}

		err = notifications.SendTournamentStartMessage(updatedTournament)
		if err != nil {
			log.Printf("Error sending tournament start message: %v", err)
		}

		// Отвечаем на callback, чтобы убрать "часики" на кнопке
		bot.Request(tgbotapi.NewCallback(callback.ID, ""))
	} else if strings.HasPrefix(callback.Data, "team_") {
		// Проверяем, был ли явно инициирован процесс добавления матча
		state, ok := teamSelectionStates[callback.From.ID]
		if !ok {
			// Если процесс добавления матча не был инициирован, игнорируем нажатие на кнопку
			return
		}
		// Обработка выбора команды
		parts := strings.Split(callback.Data, "_")
		tournamentID, _ := strconv.Atoi(parts[1])
		team := parts[2]

		// Получение текущего состояния выбора команд для пользователя
		state, ok = teamSelectionStates[callback.From.ID]
		if !ok {
			// Создание нового состояния выбора команд, если оно не существует
			state = &TeamSelectionState{
				TournamentID: tournamentID,
			}
			teamSelectionStates[callback.From.ID] = state
		}

		if state.Team1 == "" {
			// Сохранение первой выбранной команды
			state.Team1 = team

			// Отправка сообщения с запросом второй команды
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Выберите вторую команду:")
			tournament, _ := services.GetTournament(tournamentID)
			msg.ReplyMarkup = getTeamsKeyboard(tournament, []string{state.Team1}, false)
			bot.Send(msg)
		} else if state.Team2 == "" {
			// Проверка, что выбранная вторая команда не совпадает с первой командой
			if team != state.Team1 {
				// Сохранение второй выбранной команды
				state.Team2 = team

				// Отправка сообщения с запросом счета первой команды
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Введите счет первой команды:")
				bot.Send(msg)

				// Переключение состояния на ожидание ввода счета
				state.AwaitingScore = true
			} else {
				// Отправка сообщения об ошибке, если выбрана та же команда, что и первая
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Вы не можете выбрать ту же команду, что и первая. Выберите другую команду.")
				bot.Send(msg)
			}
		}
	} else if callback.Data == "confirm_delete_last_match" {
		// Получение идентификатора текущего активного турнира
		tournament, err := services.GetActiveTournament()
		if err != nil {
			log.Printf("Error getting active tournament: %v", err)
			bot.Request(tgbotapi.NewCallback(callback.ID, "Произошла ошибка при получении активного турнира."))
			return
		}
		if tournament == nil {
			bot.Request(tgbotapi.NewCallback(callback.ID, "В данный момент нет активного турнира."))
			return
		}

		// Определяем последний добавленный матч
		var lastMatch *db.Match
		var stageType string

		if tournament.Playoff != nil {
			// Турнир находится в стадии плей-офф
			if tournament.Playoff.Final != nil {
				lastMatch = tournament.Playoff.Final
				stageType = "финал"
			} else if len(tournament.Playoff.SemiFinals) > 0 {
				lastMatch = &tournament.Playoff.SemiFinals[len(tournament.Playoff.SemiFinals)-1]
				stageType = "полуфинал"
			} else if len(tournament.Playoff.QuarterFinals) > 0 {
				lastMatch = &tournament.Playoff.QuarterFinals[len(tournament.Playoff.QuarterFinals)-1]
				stageType = "четвертьфинал"
			}
		} else {
			// Турнир находится в групповом этапе
			if len(tournament.Matches) > 0 {
				lastMatch = &tournament.Matches[len(tournament.Matches)-1]
				stageType = "групповой этап"
			}
		}

		if lastMatch == nil {
			bot.Request(tgbotapi.NewCallback(callback.ID, "В турнире еще нет добавленных матчей."))
			return
		}

		// Удаление последнего добавленного матча
		err = services.DeleteLastMatch(tournament.ID, stageType)
		if err != nil {
			log.Printf("Error deleting last match: %v", err)
			bot.Request(tgbotapi.NewCallback(callback.ID, "Произошла ошибка при удалении последнего матча."))
			return
		}

		// Отправка сообщения об успешном удалении
		bot.Request(tgbotapi.NewCallback(callback.ID, "Последний матч был успешно удален."))
		bot.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "Последний матч был успешно удален."))
	} else if callback.Data == "cancel_delete_last_match" {
		// Отправка сообщения об отмене удаления
		bot.Request(tgbotapi.NewCallback(callback.ID, "Удаление последнего матча отменено."))
		bot.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "Удаление последнего матча отменено."))
	}
}

func getTeamCategoriesKeyboard(tournamentID int) (tgbotapi.InlineKeyboardMarkup, error) {
	// Получаем список всех категорий команд из базы данных
	categories, err := db.GetTeamCategories()
	if err != nil {
		log.Printf("Error getting team categories: %v", err)
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, category := range categories {
		callbackData := fmt.Sprintf("category_selected_%d_%s", tournamentID, category.Name)
		button := tgbotapi.NewInlineKeyboardButtonData(category.Name, callbackData)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{button})
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...), nil
}

func getTeamsKeyboard(tournament *db.Tournament, selectedTeams []string, disableButtons bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, participant := range tournament.Participants {
		team := tournament.ParticipantTeams[participant]

		var buttonText string
		if contains(selectedTeams, team) {
			buttonText = "✅ " + team
		} else {
			buttonText = team
		}

		var callbackData string
		if disableButtons {
			callbackData = "ignore"
		} else {
			callbackData = fmt.Sprintf("team_%d_%s", tournament.ID, team)
		}

		button := tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{button})
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func contains(slice []string, item string) bool {
	for _, value := range slice {
		if value == item {
			return true
		}
	}
	return false
}

func addTeamCategoryHandler(message *tgbotapi.Message) {
	args := strings.Split(message.CommandArguments(), ",")
	if len(args) < 2 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Usage: /add_team_category <category_name> <team1>,<team2>,..."))
		return
	}

	categoryName := strings.TrimSpace(args[0])
	teams := make([]string, len(args)-1)
	for i, team := range args[1:] {
		teams[i] = strings.TrimSpace(team)
	}

	err := db.AddTeamCategory(categoryName, teams)
	if err != nil {
		log.Printf("Error adding team category: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while adding the team category."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Team category added successfully."))
}

func removeTeamCategoryHandler(message *tgbotapi.Message) {
	categoryName := strings.TrimSpace(message.CommandArguments())
	if categoryName == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Usage: /remove_team_category <category_name>"))
		return
	}

	err := db.RemoveTeamCategory(categoryName)
	if err != nil {
		log.Printf("Error removing team category: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "An error occurred while removing the team category."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Team category removed successfully."))
}

func handleAddAdminCommand(message *tgbotapi.Message) {
	// Проверяем, является ли пользователь администратором
	isAdmin, err := db.IsAdmin(message.From.ID)
	if err != nil {
		log.Printf("Error checking admin status: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "An error occurred while checking your admin status.")
		bot.Send(msg)
		return
	}
	if !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "You don't have permission to add admins.")
		bot.Send(msg)
		return
	}

	// Получаем идентификатор пользователя из аргументов команды
	args := strings.Split(message.CommandArguments(), " ")
	if len(args) != 1 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Usage: /addadmin <user_id>")
		bot.Send(msg)
		return
	}
	userID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Invalid user ID.")
		bot.Send(msg)
		return
	}

	// Добавляем пользователя в список администраторов
	err = db.AddAdmin(userID)
	if err != nil {
		log.Printf("Error adding admin: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "An error occurred while adding the admin.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Admin added successfully.")
	bot.Send(msg)
}

func handleRemoveAdminCommand(message *tgbotapi.Message) {
	// Проверяем, является ли пользователь администратором
	isAdmin, err := db.IsAdmin(message.From.ID)
	if err != nil {
		log.Printf("Error checking admin status: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "An error occurred while checking your admin status.")
		bot.Send(msg)
		return
	}
	if !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "You don't have permission to remove admins.")
		bot.Send(msg)
		return
	}

	// Получаем идентификатор пользователя из аргументов команды
	args := strings.Split(message.CommandArguments(), " ")
	if len(args) != 1 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Usage: /removeadmin <user_id>")
		bot.Send(msg)
		return
	}
	userID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Invalid user ID.")
		bot.Send(msg)
		return
	}

	// Удаляем пользователя из списка администраторов
	err = db.RemoveAdmin(userID)
	if err != nil {
		log.Printf("Error removing admin: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "An error occurred while removing the admin.")
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Admin removed successfully.")
	bot.Send(msg)
}

func HandleDeleteTournament(message *tgbotapi.Message) {
	// Проверяем, является ли пользователь администратором
	isAdmin, err := db.IsAdmin(message.From.ID)
	if err != nil {
		log.Printf("Error checking admin status: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Failed to check admin status.")
		bot.Send(msg)
		return
	}
	if !isAdmin {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Only admins can delete tournaments.")
		bot.Send(msg)
		return
	}

	// Получаем список активных турниров
	activeTournaments, err := services.GetActiveTournaments()
	if err != nil {
		log.Printf("Error getting active tournaments: %v", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Failed to get active tournaments.")
		bot.Send(msg)
		return
	}

	if len(activeTournaments) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "No active tournaments found.")
		bot.Send(msg)
		return
	}

	// Создаем клавиатуру с кнопками для каждого активного турнира
	keyboard := tgbotapi.NewInlineKeyboardMarkup()
	for _, tournament := range activeTournaments {
		callbackData := fmt.Sprintf("delete_tournament_%d", tournament.ID)
		button := tgbotapi.NewInlineKeyboardButtonData(tournament.Name, callbackData)
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Select a tournament to delete:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func addMatchHandler(message *tgbotapi.Message) {
	// Проверка прав доступа пользователя
	isAdmin, err := db.IsAdmin(message.From.ID)
	if err != nil {
		// Обработка ошибки, если она возникла
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при проверке прав администратора."))
		return
	}

	if !isAdmin {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "У вас нет прав для добавления результатов матчей."))
		return
	}

	_, ok := teamSelectionStates[message.From.ID]
	if ok {
		msg := tgbotapi.NewMessage(message.Chat.ID, "У вас уже есть активный процесс добавления результата матча. Пожалуйста, завершите его перед началом нового.")
		bot.Send(msg)
		return
	}

	// Получение текущего активного турнира
	tournament, err := services.GetActiveTournament()
	if err != nil {
		log.Printf("Error getting active tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Ошибка при получении активного турнира."))
		return
	}
	if tournament == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "В данный момент нет активного турнира."))
		return
	}

	// Проверка статуса турнира
	if !tournament.IsActive {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Турнир еще не начался или уже завершен. Невозможно добавить результат матча."))
		return
	}

	if tournament.Playoff != nil {
		// Турнир находится в стадии плей-офф
		teams := services.GetCurrentStageTeams(tournament)
		if len(teams) == 0 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Не удалось определить команды для текущей стадии плей-офф."))
			return
		}

		// Выводим информацию о текущем матче плей-офф
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Текущий матч плей-офф: %s vs %s", teams[0], teams[1]))
		bot.Send(msg)

		// Создаем новое состояние выбора команд для пользователя
		state := &TeamSelectionState{
			TournamentID:  tournament.ID,
			Team1:         teams[0],
			Team2:         teams[1],
			AwaitingScore: true,
		}
		teamSelectionStates[message.From.ID] = state

		// Запрашиваем счет первой команды
		msg = tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Введите счет для команды %s:", teams[0]))
		bot.Send(msg)
	} else {
		// Турнир находится в групповом этапе
		_, ok := teamSelectionStates[message.From.ID]
		if ok {
			msg := tgbotapi.NewMessage(message.Chat.ID, "У вас уже есть активный процесс добавления результата матча. Пожалуйста, завершите его перед началом нового.")
			bot.Send(msg)
			return
		}

		// Создание нового состояния выбора команд для пользователя
		state := &TeamSelectionState{
			TournamentID: tournament.ID,
		}
		teamSelectionStates[message.From.ID] = state

		// Отправка сообщения с инструкцией и клавиатурой для выбора команд
		msg := tgbotapi.NewMessage(message.Chat.ID, "Выберите первую команду:")
		msg.ReplyMarkup = getTeamsKeyboard(tournament, []string{}, false)
		bot.Send(msg)
	}
}

func handleScoreInput(message *tgbotapi.Message) {
	// Получение текущего состояния выбора команд для пользователя
	state, ok := teamSelectionStates[message.From.ID]
	if !ok {
		return
	}

	switch state.ConversationState {
	case StateAwaitingScore1:
		// Извлечение введенного счета
		score, err := strconv.Atoi(message.Text)
		if err != nil || score < 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета. Пожалуйста, введите неотрицательное целое число.")
			bot.Send(msg)
			return
		}

		// Сохранение счета первой команды
		state.Score1 = score
		state.ConversationState = StateAwaitingScore2

		// Отправка сообщения с запросом счета второй команды
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Введите счет для команды %s:", state.Team2))
		bot.Send(msg)

	case StateAwaitingScore2:
		// Извлечение введенного счета
		score, err := strconv.Atoi(message.Text)
		if err != nil || score < 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета. Пожалуйста, введите неотрицательное целое число.")
			bot.Send(msg)
			return
		}

		// Сохранение счета второй команды
		state.Score2 = score

		// Получение текущего активного турнира
		tournament, err := services.GetTournament(state.TournamentID)
		if err != nil {
			log.Printf("Error getting tournament: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении турнира.")
			bot.Send(msg)
			delete(teamSelectionStates, message.From.ID)
			return
		}

		if tournament.Playoff != nil {
			// Турнир находится в стадии плей-офф
			// Проверка, закончился ли матч вничью в основное время
			if state.Score1 == state.Score2 {
				state.ConversationState = StateAwaitingOvertimeScore

				// Запрос общего счета матча после овертайма
				msg := tgbotapi.NewMessage(message.Chat.ID, "Введите общий счет матча после овертайма (команда1:команда2):")
				bot.Send(msg)
				return
			}

			// Добавление результата матча плей-офф

			currentStage, err := services.AddPlayoffMatch(state.TournamentID, state.Team1, state.Team2, state.Score1, state.Score2, false, false)
			if err != nil {
				log.Printf("Error adding playoff match: %v", err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении результата матча плей-офф.")
				bot.Send(msg)
			} else {
				msg := tgbotapi.NewMessage(message.Chat.ID, "Результат матча плей-офф успешно сохранен.")
				bot.Send(msg)

				// Получаем обновленный турнир из базы данных
				tournament, err = services.GetTournament(state.TournamentID)
				if err != nil {
					log.Printf("Error getting updated tournament: %v", err)
					msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении обновленного турнира.")
					bot.Send(msg)
				} else {

					notifications.SendPlayoffMatchResultMessage(tournament, currentStage, &db.Match{
						Team1:     state.Team1,
						Team2:     state.Team2,
						Score1:    state.Score1,
						Score2:    state.Score2,
						ExtraTime: false,
						Penalties: false,
						Counted:   true,
					})
					// Проверяем, завершился ли плей-офф
					if tournament.Playoff.Winner != "" {
						// Плей-офф завершился, объявляем победителя
						msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Плей-офф завершился! Победитель: %s", tournament.Playoff.Winner))
						bot.Send(msg)
					} else {
						// Плей-офф продолжается, сообщаем о следующем матче
						var nextMatch string
						switch tournament.Playoff.CurrentStage {
						case "semi":
							nextMatch = "Полуфинал"
						case "final":
							nextMatch = "Финал"
						default:
							nextMatch = ""
						}
						teams := services.GetCurrentStageTeams(tournament)
						if len(teams) >= 2 {
							msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Следующий матч (%s): %s vs %s", nextMatch, teams[0], teams[1]))
							bot.Send(msg)
						} else {
							// Если нет информации о следующем матче, отправляем сообщение о продолжении плей-офф
							msg := tgbotapi.NewMessage(message.Chat.ID, "Плей-офф продолжается. Ожидайте информацию о следующем матче.")
							bot.Send(msg)
						}
					}
				}
			}
		} else {
			// Турнир находится в групповом этапе
			// Проверка наличия уже добавленного результата матча
			existingMatch, err := getMatchResult(state.TournamentID, state.Team1, state.Team2)
			if err == nil && existingMatch != nil {
				msg := tgbotapi.NewMessage(message.Chat.ID, "Результат матча между этими командами уже был добавлен ранее.")
				bot.Send(msg)
				delete(teamSelectionStates, message.From.ID)
				return
			}

			// Добавление результата матча в групповой этап
			err = services.AddMatchResult(state.TournamentID, state.Team1, state.Team2, state.Score1, state.Score2)
			if err != nil {
				log.Printf("Error adding match result: %v", err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении результата матча.")
				bot.Send(msg)
			} else {
				msg := tgbotapi.NewMessage(message.Chat.ID, "Результат матча успешно сохранен.")
				bot.Send(msg)
			}
		}

		// Сброс состояния выбора команд
		delete(teamSelectionStates, message.From.ID)

	case StateAwaitingOvertimeScore:
		// Обработка общего счета матча после овертайма
		overtimeScoreParts := strings.Split(message.Text, ":")
		if len(overtimeScoreParts) != 2 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета овертайма. Введите общий счет матча после овертайма (команда1:команда2).")
			bot.Send(msg)
			return
		}
		overtimeScore1, err := strconv.Atoi(overtimeScoreParts[0])
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета первой команды в овертайме. Введите общий счет матча после овертайма (команда1:команда2).")
			bot.Send(msg)
			return
		}
		overtimeScore2, err := strconv.Atoi(overtimeScoreParts[1])
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета второй команды в овертайме. Введите общий счет матча после овертайма (команда1:команда2).")
			bot.Send(msg)
			return
		}
		state.Score1 = overtimeScore1
		state.Score2 = overtimeScore2

		// Проверка, закончился ли матч вничью после овертайма
		if state.Score1 == state.Score2 {
			state.ConversationState = StateAwaitingPenaltiesScore

			// Запрос счета серии пенальти
			msg := tgbotapi.NewMessage(message.Chat.ID, "Введите счет серии пенальти (команда1:команда2):")
			bot.Send(msg)
			return
		}

		// Добавление результата матча плей-офф с овертаймом
		currentStage, err := services.AddPlayoffMatch(state.TournamentID, state.Team1, state.Team2, state.Score1, state.Score2, true, false)
		if err != nil {
			log.Printf("Error adding playoff match: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении результата матча плей-офф.")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Результат матча плей-офф успешно сохранен.")
			bot.Send(msg)

			// Получаем обновленный турнир из базы данных
			tournament, err := services.GetTournament(state.TournamentID)
			if err != nil {
				log.Printf("Error getting updated tournament: %v", err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении обновленного турнира.")
				bot.Send(msg)
			} else {

				match := &db.Match{
					Team1:     state.Team1,
					Team2:     state.Team2,
					Score1:    state.Score1,
					Score2:    state.Score2,
					ExtraTime: true,
					Penalties: false,
					Counted:   true,
				}

				// Отправляем уведомление о результате матча плей-офф с овертаймом
				err = notifications.SendPlayoffMatchResultMessage(tournament, currentStage, match)
				if err != nil {
					log.Printf("Error sending playoff match result message: %v", err)
				}
				// Проверяем, завершился ли плей-офф
				if tournament.Playoff.Winner != "" {
					// Плей-офф завершился, объявляем победителя
					msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Плей-офф завершился! Победитель: %s", tournament.Playoff.Winner))
					bot.Send(msg)
				} else {
					// Плей-офф продолжается, сообщаем о следующем матче
					var nextMatch string
					switch tournament.Playoff.CurrentStage {
					case "semi":
						nextMatch = "Полуфинал"
					case "final":
						nextMatch = "Финал"
					default:
						nextMatch = ""
					}
					teams := services.GetCurrentStageTeams(tournament)
					if len(teams) >= 2 {
						msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Следующий матч (%s): %s vs %s", nextMatch, teams[0], teams[1]))
						bot.Send(msg)
					} else {
						// Если нет информации о следующем матче, отправляем сообщение о продолжении плей-офф
						msg := tgbotapi.NewMessage(message.Chat.ID, "Плей-офф продолжается. Ожидайте информацию о следующем матче.")
						bot.Send(msg)
					}
				}
			}
		}

		// Сброс состояния выбора команд
		delete(teamSelectionStates, message.From.ID)

	case StateAwaitingPenaltiesScore:
		// Обработка счета серии пенальти
		penaltiesScoreParts := strings.Split(message.Text, ":")
		if len(penaltiesScoreParts) != 2 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета серии пенальти. Введите счет серии пенальти (команда1:команда2).")
			bot.Send(msg)
			return
		}
		penaltiesScore1, err := strconv.Atoi(penaltiesScoreParts[0])
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета первой команды в серии пенальти. Введите счет серии пенальти (команда1:команда2).")
			bot.Send(msg)
			return
		}
		penaltiesScore2, err := strconv.Atoi(penaltiesScoreParts[1])
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат счета второй команды в серии пенальти. Введите счет серии пенальти (команда1:команда2).")
			bot.Send(msg)
			return
		}
		state.Score1 = penaltiesScore1
		state.Score2 = penaltiesScore2

		// Добавление результата матча плей-офф с овертаймом и серией пенальти
		currentStage, err := services.AddPlayoffMatch(state.TournamentID, state.Team1, state.Team2, state.Score1, state.Score2, true, true)
		if err != nil {
			log.Printf("Error adding playoff match: %v", err)
			msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при сохранении результата матча плей-офф.")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Результат матча плей-офф успешно сохранен.")
			bot.Send(msg)

			// Получаем обновленный турнир из базы данных
			tournament, err := services.GetTournament(state.TournamentID)
			if err != nil {
				log.Printf("Error getting updated tournament: %v", err)
				msg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении обновленного турнира.")
				bot.Send(msg)
			} else {

				match := &db.Match{
					Team1:     state.Team1,
					Team2:     state.Team2,
					Score1:    state.Score1,
					Score2:    state.Score2,
					ExtraTime: true,
					Penalties: true,
					Counted:   true,
				}

				// Отправляем уведомление о результате матча плей-офф с овертаймом
				err = notifications.SendPlayoffMatchResultMessage(tournament, currentStage, match)
				if err != nil {
					log.Printf("Error sending playoff match result message: %v", err)
				}
				// Проверяем, завершился ли плей-офф
				if tournament.Playoff.Winner != "" {
					// Плей-офф завершился, объявляем победителя
					msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Плей-офф завершился! Победитель: %s", tournament.Playoff.Winner))
					bot.Send(msg)
				} else {
					// Плей-офф продолжается, сообщаем о следующем матче
					var nextMatch string
					switch tournament.Playoff.CurrentStage {
					case "semi":
						nextMatch = "Полуфинал"
					case "final":
						nextMatch = "Финал"
					default:
						nextMatch = ""
					}
					teams := services.GetCurrentStageTeams(tournament)
					if len(teams) >= 2 {
						msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Следующий матч (%s): %s vs %s", nextMatch, teams[0], teams[1]))
						bot.Send(msg)
					} else {
						// Если нет информации о следующем матче
						msg := tgbotapi.NewMessage(message.Chat.ID, "Плей-офф продолжается. Ожидайте информацию о следующем матче.")
						bot.Send(msg)
					}
				}
			}
		}

		// Сброс состояния выбора команд
		delete(teamSelectionStates, message.From.ID)
	}
}

func getNextStage(currentStage string) string {
	switch currentStage {
	case "quarter":
		return "semi"
	case "semi":
		return "final"
	default:
		return ""
	}
}

func getMatchResult(tournamentID int, team1, team2 string) (*db.Match, error) {
	filter := bson.M{
		"id": tournamentID,
		"matches": bson.M{
			"$elemMatch": bson.M{
				"team1": team1,
				"team2": team2,
			},
		},
	}

	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), filter).Decode(&tournament)
	if err != nil {
		return nil, err
	}

	for _, match := range tournament.Matches {
		if match.Team1 == team1 && match.Team2 == team2 {
			return &match, nil
		}
	}

	return nil, errors.New("match not found")
}

func tournamentInfoHandler(message *tgbotapi.Message) {
	// Получение идентификатора текущего активного турнира
	tournament, err := services.GetActiveTournament()
	if err != nil {
		log.Printf("Error getting active tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении активного турнира."))
		return
	}
	if tournament == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "В данный момент нет активного турнира."))
		return
	}

	// Формирование сообщения с турнирной таблицей и списком матчей
	messageParts := []string{
		fmt.Sprintf("Турнирная таблица для турнира *%s*:\n", tournament.Name),
		"```",
		"Место | Команда (Игрок)      | И | В | Н | П | ЗГ | ПГ | РГ  | О",
		strings.Repeat("-", 54),
	}

	// Создание карты для хранения соответствия команд и игроков
	teamPlayerMap := make(map[string]string)
	for player, team := range tournament.ParticipantTeams {
		teamPlayerMap[team] = player
	}

	// Создание карты для хранения статистики команд
	teamStandingsMap := make(map[string]db.Standing)
	for _, standing := range tournament.Standings {
		teamStandingsMap[standing.Team] = standing
	}

	// Сортировка команд по критериям
	standings := tournament.Standings

	sort.Slice(standings, func(i, j int) bool {
		// Сравнение по количеству очков
		if standings[i].Points != standings[j].Points {
			return standings[i].Points > standings[j].Points
		}
		// Сравнение по разнице забитых и пропущенных мячей
		if standings[i].GoalsDifference != standings[j].GoalsDifference {
			return standings[i].GoalsDifference > standings[j].GoalsDifference
		}
		// Сравнение по количеству забитых мячей
		if standings[i].GoalsFor != standings[j].GoalsFor {
			return standings[i].GoalsFor > standings[j].GoalsFor
		}
		// Сравнение по количеству сыгранных матчей
		if standings[i].Played != standings[j].Played {
			return standings[i].Played > standings[j].Played
		}
		// Сравнение по результатам личных встреч
		headToHeadResult := getHeadToHeadResult(standings[i].Team, standings[j].Team, tournament.Matches)
		if headToHeadResult != 0 {
			return headToHeadResult > 0
		}
		// Сравнение по алфавиту
		return standings[i].Team < standings[j].Team
	})

	// Вывод информации о командах в турнирной таблице
	position := 1
	for _, standing := range standings {
		player, ok := teamPlayerMap[standing.Team]
		if !ok {
			continue
		}

		// Форматирование строки с информацией о команде и игроке
		teamPlayerInfo := fmt.Sprintf("%s (%s)", standing.Team, player)

		messageParts = append(messageParts, fmt.Sprintf(
			"%-5d | %-20s | %d | %d | %d | %d | %2d | %2d | %+3d | %d",
			position, teamPlayerInfo,
			standing.Played, standing.Won, standing.Drawn, standing.Lost,
			standing.GoalsFor, standing.GoalsAgainst, standing.GoalsDifference, standing.Points,
		))

		position++
	}

	messageParts = append(messageParts, "```")

	messageParts = append(messageParts, "\nСписок матчей:\n")
	if len(tournament.Matches) == 0 {
		messageParts = append(messageParts, "Пока нет сыгранных матчей.")
	} else {
		for _, match := range tournament.Matches {
			messageParts = append(messageParts, fmt.Sprintf(
				"%s %d - %d %s",
				match.Team1, match.Score1, match.Score2, match.Team2,
			))
		}
	}

	// Добавляем информацию о плей-офф
	messageParts = append(messageParts, "\nПлей-офф:\n")

	if tournament.Playoff != nil {
		// Выводим информацию о четвертьфиналах
		if len(tournament.Playoff.QuarterFinals) > 0 {
			messageParts = append(messageParts, "Четвертьфиналы:\n")
			for _, match := range tournament.Playoff.QuarterFinals {
				messageParts = append(messageParts, fmt.Sprintf(
					"%s vs %s\n",
					match.Team1, match.Team2,
				))
			}
		}

		// Выводим информацию о полуфиналах
		if len(tournament.Playoff.SemiFinals) > 0 {
			messageParts = append(messageParts, "\nПолуфиналы:\n")
			for _, match := range tournament.Playoff.SemiFinals {
				messageParts = append(messageParts, fmt.Sprintf(
					"%s vs %s\n",
					match.Team1, match.Team2,
				))
			}
		}

		// Выводим информацию о финале
		if tournament.Playoff.Final != nil {
			messageParts = append(messageParts, "\nФинал:\n")
			messageParts = append(messageParts, fmt.Sprintf(
				"%s vs %s\n",
				tournament.Playoff.Final.Team1, tournament.Playoff.Final.Team2,
			))
		}

		// Выводим информацию о победителе
		if tournament.Playoff.Winner != "" {
			messageParts = append(messageParts, fmt.Sprintf("\nПобедитель: %s\n", tournament.Playoff.Winner))
		}
	} else {
		messageParts = append(messageParts, "Плей-офф еще не начался.")
	}

	// Отправка сообщения с турнирной таблицей, списком матчей и информацией о плей-офф
	msg := tgbotapi.NewMessage(message.Chat.ID, strings.Join(messageParts, "\n"))
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// Добавьте эту функцию для удаления клавиатуры
func removeKeyboard(chatID int64, messageID int) {
	msg := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, tgbotapi.InlineKeyboardMarkup{})
	bot.Send(msg)
}

func startPlayoffHandler(message *tgbotapi.Message) {
	// Получение идентификатора текущего активного турнира
	tournament, err := services.GetActiveTournament()
	if err != nil {
		log.Printf("Error getting active tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении активного турнира."))
		return
	}
	if tournament == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "В данный момент нет активного турнира."))
		return
	}

	// Проверяем, что плей-офф еще не начался
	if tournament.Playoff != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Плей-офф уже начался."))
		return
	}

	// Начинаем плей-офф
	err = services.StartPlayoff(tournament.ID)
	if err != nil {
		log.Printf("Error starting playoff: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при начале плей-офф."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Плей-офф начался!"))
}

func getHeadToHeadResult(team1, team2 string, matches []db.Match) int {
	team1Wins := 0
	team2Wins := 0

	for _, match := range matches {
		if match.Team1 == team1 && match.Team2 == team2 {
			if match.Score1 > match.Score2 {
				team1Wins++
			} else if match.Score1 < match.Score2 {
				team2Wins++
			}
		} else if match.Team1 == team2 && match.Team2 == team1 {
			if match.Score1 > match.Score2 {
				team2Wins++
			} else if match.Score1 < match.Score2 {
				team1Wins++
			}
		}
	}

	if team1Wins > team2Wins {
		return 1
	} else if team1Wins < team2Wins {
		return -1
	} else {
		return 0
	}
}

func deleteLastMatchHandler(message *tgbotapi.Message) {
	// Получение идентификатора текущего активного турнира
	tournament, err := services.GetActiveTournament()
	if err != nil {
		log.Printf("Error getting active tournament: %v", err)
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при получении активного турнира."))
		return
	}
	if tournament == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "В данный момент нет активного турнира."))
		return
	}

	var lastMatch *db.Match
	var stageType string

	if tournament.Playoff != nil {
		// Турнир находится в стадии плей-офф
		if tournament.Playoff.Final != nil {
			lastMatch = tournament.Playoff.Final
			stageType = "финал"
		} else if len(tournament.Playoff.SemiFinals) > 0 {
			lastMatch = &tournament.Playoff.SemiFinals[len(tournament.Playoff.SemiFinals)-1]
			stageType = "полуфинал"
		} else if len(tournament.Playoff.QuarterFinals) > 0 {
			lastMatch = &tournament.Playoff.QuarterFinals[len(tournament.Playoff.QuarterFinals)-1]
			stageType = "четвертьфинал"
		}
	} else {
		// Турнир находится в групповом этапе
		if len(tournament.Matches) > 0 {
			lastMatch = &tournament.Matches[len(tournament.Matches)-1]
			stageType = "групповой этап"
		}
	}

	if lastMatch == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "В турнире еще нет добавленных матчей."))
		return
	}

	// Отправка предупреждения перед удалением
	warningMessage := fmt.Sprintf("Вы уверены, что хотите удалить последний добавленный матч?\n\nСтадия: %s\nМатч: %s vs %s\nСчет: %d - %d",
		stageType, lastMatch.Team1, lastMatch.Team2, lastMatch.Score1, lastMatch.Score2)
	confirmKeyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Да", "confirm_delete_last_match"),
			tgbotapi.NewInlineKeyboardButtonData("Нет", "cancel_delete_last_match"),
		),
	)
	msg := tgbotapi.NewMessage(message.Chat.ID, warningMessage)
	msg.ReplyMarkup = confirmKeyboard
	bot.Send(msg)
}
