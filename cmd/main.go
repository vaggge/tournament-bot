package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mongodb"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/robfig/cron/v3"
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

	m, err := migrate.New("file://migrations", "mongodb://mongo:27017/tournament")
	if err != nil {
		log.Fatal(err)
	}
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		log.Fatal(err)
	}

	// Создаем новый планировщик задач
	c := cron.New()

	// Добавляем задачу для удаления незавершенных турниров каждый день в 6:00 AM по московскому времени
	_, err = c.AddFunc("0 6 * * *", deleteUnfinishedTournaments)
	if err != nil {
		log.Fatalf("Error adding deleteUnfinishedTournaments to cron: %v", err)
	}

	// Запускаем планировщик задач
	c.Start()

	// Запуск веб-сервера для обработки вебхуков
	go web.StartServer(":8081")

	go deleteUnfinishedTournaments()

	// Настройка вебхука
	err = bot.SetWebhook("7012505888:AAEtQoe-AwaoNoC5OPUQaQ6jAqNHKYAKcQk", "https://tournament-bot.online/webhook")
	if err != nil {
		log.Fatalf("Error setting webhook: %v", err)
	}

	botAPI, err := tgbotapi.NewBotAPI("7012505888:AAEtQoe-AwaoNoC5OPUQaQ6jAqNHKYAKcQk")
	if err != nil {
		log.Fatal(err)
	}

	// Установка меню команд
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "🚀 Запустить бота"},
		{Command: "create_tournament", Description: "🏆 Создать новый турнир (только для админов)"},
		{Command: "delete_tournament", Description: "🗑️ Удалить активный турнир (только для админов)"},
		{Command: "addadmin", Description: "👤 Добавить нового администратора (только для админов)"},
		{Command: "tournament_info", Description: "ℹ️ Показать турнирную таблицу и матчи"},
		{Command: "cancel", Description: "❌ Отменить добавление матча"},
		{Command: "start_playoff", Description: "🔥 Начать этап плей-офф турнира"},
		{Command: "deletelastmatch", Description: "🗑️ Удалить последний добавленный матч (только для админов)"},
		{Command: "add_match", Description: "➕ Добавить результат матча (только для админов)"},
	}

	_, err = botAPI.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Printf("Error setting command menu: %v", err)
	}

	// Ожидание завершения программы
	select {}
}

func deleteUnfinishedTournaments() {
	// Получаем список всех незавершенных и неактивных турниров
	inactiveTournaments, err := services.GetInactiveTournaments()
	if err != nil {
		log.Printf("Error getting inactive tournaments: %v", err)
		return
	}

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
