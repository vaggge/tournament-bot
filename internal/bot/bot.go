package bot

import (
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"net/http"
)

var bot *tgbotapi.BotAPI

func SetWebhook(token, webhookURL string) error {
	var err error
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}

	// Установка вебхука
	wh, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		return err
	}

	_, err = bot.Request(wh)
	if err != nil {
		return err
	}

	fmt.Println("Webhook set successfully")
	return nil
}

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	var update tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if update.Message != nil {
		HandleMessage(update.Message)
	} else if update.CallbackQuery != nil {
		СallbackHandler(update.CallbackQuery)
	}
}

func Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return bot.Send(c)
}
