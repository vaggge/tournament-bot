package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"log"
	"time"
	"tournament-bot/internal/bot"
	"tournament-bot/internal/db"
	"tournament-bot/internal/services"
	"tournament-bot/internal/web"
)

func main() {
	// Инициализация базы данных
	db.InitDB()

	m, err := migrate.New("file://migrations", "mongodb://localhost:27017/tournament")
	if err != nil {
		log.Fatal(err)
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		log.Fatal(err)
	}

	// Запуск веб-сервера для обработки вебхуков
	go web.StartServer(":8080")

	go deleteUnfinishedTournaments()

	// Настройка вебхука
	err = bot.SetWebhook("7012505888:AAEtQoe-AwaoNoC5OPUQaQ6jAqNHKYAKcQk", "https://ca41-46-159-186-86.ngrok-free.app/webhook")
	if err != nil {
		log.Fatalf("Error setting webhook: %v", err)
	}

	botAPI, err := tgbotapi.NewBotAPI("7012505888:AAEtQoe-AwaoNoC5OPUQaQ6jAqNHKYAKcQk")
	if err != nil {
		log.Fatal(err)
	}

	// Установка меню команд
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Start the bot"},
		{Command: "create_tournament", Description: "Create a new tournament (admin only)"},
		{Command: "delete_tournament", Description: "Delete an active tournament (admin only)"},
		{Command: "join", Description: "Join the current tournament"},
		{Command: "leave", Description: "Leave the current tournament"},
		{Command: "teams", Description: "Select teams for the tournament"},
		{Command: "draw", Description: "Perform team draw for the tournament"},
		{Command: "start_tournament", Description: "Start the tournament"},
		{Command: "help", Description: "Show available commands"},
		{Command: "addadmin", Description: "Add a new admin (admin only)"},
		{Command: "removeadmin", Description: "Remove an admin (admin only)"},
		{Command: "addteamcategory", Description: "Add a new team category (admin only)"},
		{Command: "removeteamcategory", Description: "Remove a team category (admin only)"},
		{Command: "tournament_info", Description: "Show tournament standings and matches"},
		{Command: "cancel", Description: "Прервать добавление матча"},
		{Command: "start_playoff", Description: "Start the playoff stage of the tournament"},
	}

	_, err = botAPI.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Printf("Error setting command menu: %v", err)
	}

	// Ожидание завершения программы
	select {}
}

func deleteUnfinishedTournaments() {
	// Настройка времени запуска задачи (6:00 AM по московскому времени)
	moscowLocation, _ := time.LoadLocation("Europe/Moscow")
	startTime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 6, 0, 0, 0, moscowLocation)

	// Если текущее время уже позже 6:00 AM, переносим запуск на следующий день
	if time.Now().After(startTime) {
		startTime = startTime.AddDate(0, 0, 1)
	}

	// Вычисляем время до запуска задачи
	duration := time.Until(startTime)

	// Ждем до наступления времени запуска
	time.Sleep(duration)

	// Бесконечный цикл для ежедневного запуска задачи
	for {
		// Получаем список всех незавершенных и неактивных турниров
		inactiveTournaments, err := services.GetInactiveTournaments()
		if err != nil {
			log.Printf("Error getting inactive tournaments: %v", err)
		} else {
			// Проверяем каждый неактивный турнир
			for _, tournament := range inactiveTournaments {
				if (!tournament.SetupCompleted || !tournament.IsActive) && time.Since(tournament.CreatedAt) > 24*time.Hour {
					// Если настройка турнира не завершена или турнир неактивен, и прошло более 24 часов с момента создания, удаляем турнир
					err := services.DeleteTournament(tournament.ID)
					if err != nil {
						log.Printf("Error deleting tournament: %v", err)
					}
				}
			}
		}

		// Ждем 1 час до следующей проверки
		time.Sleep(1 * time.Hour)
	}
}
