//go:generate protoc -I msg --go_out=plugins=grpc:msg msg/msg.proto
package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tb "github.com/go-telegram-bot-api/telegram-bot-api"

	pb "github.com/usher2/u2ckbot/msg"
)

func botUpdates(c pb.CheckClient, bot *tb.BotAPI, updatesChan tb.UpdatesChannel) {
	for {
		select {
		case update := <-updatesChan:
			if update.Message != nil { // ignore any non-Message Updates
				if update.Message.Text != "" {
					if update.Message.Chat.Type == "private" ||
						(update.Message.ReplyToMessage == nil &&
							update.Message.ForwardFromMessageID == 0) {
						var uname string
						// who writing
						if update.Message.From != nil {
							uname = update.Message.From.UserName
						}
						// chat/dialog
						chat := update.Message.Chat
						go Talks(c, bot, uname, chat, "", 0, "", update.Message.Text)
					}
				}
			} else if update.CallbackQuery != nil {
				var (
					uname string
					req   string
				)
				// who writing
				if update.CallbackQuery.From != nil {
					uname = update.CallbackQuery.From.UserName
				}
				// chat/dialog
				var chat *tb.Chat
				if update.CallbackQuery.Message != nil {
					chat = update.CallbackQuery.Message.Chat
					i := strings.IndexByte(update.CallbackQuery.Message.Text, '\n')
					if i > 0 {
						if strings.HasPrefix(update.CallbackQuery.Message.Text[:i], "\U0001f525 ") &&
							strings.HasSuffix(update.CallbackQuery.Message.Text[:i], " заблокирован") {
							req = strings.TrimSuffix(strings.TrimPrefix(update.CallbackQuery.Message.Text[:i], "\U0001f525 "), " заблокирован")
						}
					}
				}
				go Talks(c, bot, uname, chat, "", update.CallbackQuery.Message.MessageID, update.CallbackQuery.Data, req)
			} else if update.InlineQuery != nil {
				if update.InlineQuery.Query != "" {
					var uname string
					// who writing
					if update.InlineQuery.From != nil {
						uname = update.InlineQuery.From.UserName
					}
					go Talks(c, bot, uname, nil, update.InlineQuery.ID, 0, "", update.InlineQuery.Query)
				}
			}
		}
	}
}

var noAdCount int = 0

const NO_AD_NUMBER = 20

func makePagination(offset TPagination, pages []TPagination) tb.InlineKeyboardMarkup {
	var (
		keyboard [][]tb.InlineKeyboardButton
		o        int
	)
	for i, _ := range pages {
		if pages[i].Tag == OFFSET_CONTENT {
			if pages[i].Count > PRINT_LIMIT {
				row := tb.NewInlineKeyboardRow()
				if offset.Tag != OFFSET_CONTENT {
					o = 0
				} else {
					o = offset.Count
				}
				if pages[i].Count > 2*PRINT_LIMIT {
					row = append(row,
						tb.NewInlineKeyboardButtonData(fmt.Sprintf("<< %d", 1),
							fmt.Sprintf("%d:%d", OFFSET_CONTENT, 0)),
					)
				}
				_o := o - PRINT_LIMIT
				if _o < 0 {
					_o = 0
				}
				row = append(row,
					tb.NewInlineKeyboardButtonData(fmt.Sprintf("< %d", _o/PRINT_LIMIT+1),
						fmt.Sprintf("%d:%d", OFFSET_CONTENT, _o)),
				)
				row = append(row,
					tb.NewInlineKeyboardButtonData(fmt.Sprintf("- %d -", o/PRINT_LIMIT+1),
						fmt.Sprintf("%d:%d", OFFSET_CONTENT, o)),
				)
				_o = o + PRINT_LIMIT
				if _o > pages[i].Count-(pages[i].Count%PRINT_LIMIT) {
					_o = pages[i].Count - (pages[i].Count % PRINT_LIMIT)
				}
				if _o == pages[i].Count {
					_o -= PRINT_LIMIT
				}
				_p := _o/PRINT_LIMIT + 1
				row = append(row,
					tb.NewInlineKeyboardButtonData(fmt.Sprintf("%d >", _p),
						fmt.Sprintf("%d:%d", OFFSET_CONTENT, _o)),
				)
				if pages[i].Count > 2*PRINT_LIMIT {
					_o = pages[i].Count - (pages[i].Count % PRINT_LIMIT)
					if _o == pages[i].Count {
						_o -= PRINT_LIMIT
					}
					_p = _o/PRINT_LIMIT + 1
					row = append(row,
						tb.NewInlineKeyboardButtonData(fmt.Sprintf("%d >>", _p),
							fmt.Sprintf("%d:%d", OFFSET_CONTENT,
								_o)),
					)
				}
				keyboard = append(keyboard, row)
			}
		}
	}
	return tb.InlineKeyboardMarkup{
		InlineKeyboard: keyboard,
	}
}

func sendMessage(bot *tb.BotAPI, chat *tb.Chat, inlineId string, messageId int, text string, offset TPagination, pages []TPagination) {
	if chat != nil {
		if noAdCount >= NO_AD_NUMBER {
			text += "--- \n" + DonateFooter
			noAdCount = 0
		} else {
			//text += Footer
			noAdCount += 1
		}
		if messageId > 0 {
			msg := tb.NewEditMessageText(chat.ID, messageId, text)
			msg.ParseMode = tb.ModeMarkdown
			msg.DisableWebPagePreview = true
			inlineKeyboard := makePagination(offset, pages)
			if len(inlineKeyboard.InlineKeyboard) > 0 {
				msg.ReplyMarkup = &inlineKeyboard
			}
			_, err := bot.Send(msg)
			if err != nil {
				Warning.Printf("Error sending message: %s\n", err.Error())
			}
		} else {
			msg := tb.NewMessage(chat.ID, text)
			msg.ParseMode = tb.ModeMarkdown
			msg.DisableWebPagePreview = true
			inlineKeyboard := makePagination(offset, pages)
			if len(inlineKeyboard.InlineKeyboard) > 0 {
				msg.ReplyMarkup = inlineKeyboard
			}
			_, err := bot.Send(msg)
			if err != nil {
				Warning.Printf("Error sending message: %s\n", err.Error())
			}
		}
	} else if inlineId != "" {
		article := tb.InlineQueryResultArticle{
			ID:    inlineId,
			Title: "Search result",
			Type:  "article",
			InputMessageContent: tb.InputTextMessageContent{
				Text:                  text + Footer,
				ParseMode:             tb.ModeMarkdown,
				DisableWebPagePreview: true,
			},
		}
		inlineConf := tb.InlineConfig{
			InlineQueryID: inlineId,
			Results:       []interface{}{article},
		}
		if _, err := bot.AnswerInlineQuery(inlineConf); err != nil {
			Warning.Printf("Error sending answer: %s\n", err.Error())
		}
	}
}

// Handle commands
func Talks(c pb.CheckClient, bot *tb.BotAPI, uname string, chat *tb.Chat, inlineId string, messageId int, callbackData, text string) {
	var (
		reply  string
		pages  []TPagination
		offset TPagination
	)
	if callbackData != "" && strings.IndexByte(callbackData, ':') != -1 {
		i := strings.IndexByte(callbackData, ':')
		if i != len(callbackData)-1 {
			offset.Tag, _ = strconv.Atoi(callbackData[:i])
			offset.Count, _ = strconv.Atoi(callbackData[i+1:])
			Debug.Println("!!!!!!!", callbackData, offset)
		}
	}
	if chat != nil {
		bot.Send(tb.NewChatAction(chat.ID, "typing"))
	}
	//log.Printf("[%s] %d %s", UserName, ChatID, Text)
	regex, _ := regexp.Compile(`^/([A-Za-z\_]+)\s*(.*)$`)
	matches := regex.FindStringSubmatch(text)
	// hanlde chat commands
	if len(matches) > 0 {
		comm := matches[1]
		commArgs := []string{""}
		if len(matches) >= 3 {
			commArgs = regexp.MustCompile(`\s+`).Split(matches[2], -1)
			if bot.Self.UserName != "" {
				for i, s := range commArgs {
					commArgs[i] = strings.TrimSuffix(s, "@"+bot.Self.UserName)
				}
			}
		}
		switch comm {
		case `help`:
			reply = HelpMessage
		case `helpen`:
			reply = HelpMessageEn
		case `donate`:
			reply = DonateMessage
		case `n_`, `ck`, `check`:
			if len(commArgs) > 0 {
				reply, pages = mainSearch(c, commArgs[0], offset)
			} else {
				reply = "😱Нечего искать\n"
			}
		case `start`:
			reply = "Приветствую тебя, " + uname + "!\n"
			//case `ping`:
			//	reply = Ping(c)
			//default:
			//	reply = "😱 Unknown command\n"
		}
		if reply != "" {
			sendMessage(bot, chat, inlineId, messageId, reply, offset, pages)
		}
	} else {
		if len(text) > 0 && text[0] != '/' {
			reply, pages = mainSearch(c, text, offset)
			sendMessage(bot, chat, inlineId, messageId, reply, offset, pages)
		} else {
			reply = "😱Нечего искать\n"
			sendMessage(bot, chat, inlineId, messageId, reply, offset, pages)
		}
	}
}
