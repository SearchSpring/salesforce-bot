package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/kelseyhightower/envconfig"
	"github.com/nlopes/slack"

	"github.com/searchspring/nebo/nextopia"
	"github.com/searchspring/nebo/salesforce"
)

type envVars struct {
	DevMode                string `split_words:"true" required:"true"`
	SlackVerificationToken string `split_words:"true" required:"true"`
	SlackOauthToken        string `split_words:"true" required:"true"`
	SfURL                  string `split_words:"true" required:"true"`
	SfUser                 string `split_words:"true" required:"true"`
	SfPassword             string `split_words:"true" required:"true"`
	SfToken                string `split_words:"true" required:"true"`
	NxUser                 string `split_words:"true" required:"true"`
	NxPassword             string `split_words:"true" required:"true"`
	GdriveFireDocFolderID  string `split_words:"true" required:"true"`
}

var salesForceDAO salesforce.DAO = nil
var nextopiaDAO nextopia.DAO = nil

// Handler - check routing and call correct methods
func Handler(w http.ResponseWriter, r *http.Request) {
	var env envVars
	err := envconfig.Process("", &env)
	if err != nil {
		sendInternalServerError(w, err)
		return
	}

	blanks := findBlankEnvVars(env)
	if len(blanks) > 0 {
		err := fmt.Errorf("the following env vars are blank: %s", strings.Join(blanks, ", "))
		if env.DevMode != "development" {
			sendInternalServerError(w, err)
			return
		}
		log.Printf(err.Error())
	}

	s, err := slack.SlashCommandParse(r)
	if err != nil {
		sendInternalServerError(w, err)
		return
	}

	if !s.ValidateToken(env.SlackVerificationToken) {
		err := errors.New("slack verification failed")
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	nextopiaDAO = nextopia.NewDAO(env.NxUser, env.NxPassword)
	salesForceDAO = salesforce.NewDAO(env.SfURL, env.SfUser, env.SfPassword, env.SfToken)

	w.Header().Set("Content-type", "application/json")
	switch s.Command {
	case "/rep", "/alpha-nebo", "/nebo":
		if strings.TrimSpace(s.Text) == "help" || strings.TrimSpace(s.Text) == "" {
			writeHelpNebo(w)
			return
		}
		if salesForceDAO == nil {
			sendInternalServerError(w, errors.New("missing required Salesforce credentials"))
			return
		}
		responseJSON, err := salesForceDAO.Query(s.Text)
		if err != nil {
			sendInternalServerError(w, err)
			return
		}
		w.Write(responseJSON)
		return

	case "/fire", "/firetest":
		if strings.TrimSpace(s.Text) == "help" {
			writeHelpFire(w)
			return
		}
		fireResponse(env.GdriveFireDocFolderID, s.ResponseURL)
		return

	case "/firedown":
		w.Write(fireDownResponse())
		return

	case "/neboidnx", "/neboid":
		if strings.TrimSpace(s.Text) == "help" || strings.TrimSpace(s.Text) == "" {
			writeHelpNeboid(w)
			return
		}
		if nextopiaDAO == nil {
			sendInternalServerError(w, errors.New("missing required Nextopia credentials"))
			return
		}
		responseJSON, err := nextopiaDAO.Query(s.Text)
		if err != nil {
			sendInternalServerError(w, err)
			return
		}
		w.Write(responseJSON)
		return

	case "/neboidss":
		if strings.TrimSpace(s.Text) == "help" || strings.TrimSpace(s.Text) == "" {
			writeHelpNeboid(w)
			return
		}
		if salesForceDAO == nil {
			sendInternalServerError(w, errors.New("missing required Salesforce credentials"))
			return
		}
		responseJSON, err := salesForceDAO.IDQuery(s.Text)
		if err != nil {
			sendInternalServerError(w, err)
			return
		}
		w.Write(responseJSON)
		return

	case "/meet":
		if strings.TrimSpace(s.Text) == "help" {
			writeHelpMeet(w)
			return
		}
		responseJSON := meetResponse(s.Text)
		w.Write(responseJSON)
		return

	case "/meettest":
		if strings.TrimSpace(s.Text) == "help" {
			writeHelpMeet(w)
			return
		}
		responseJSON := meetResponse(s.Text)
		w.Write(responseJSON)
		return

	default:
		sendInternalServerError(w, errors.New("unknown slash command "+s.Command))
		return
	}
}

func writeHelpFire(w http.ResponseWriter) {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "Fire usage:\n`/fire` - generate a fire checklist to handle the fire",
	}
	json, _ := json.Marshal(msg)
	w.Write(json)
}

func writeHelpNebo(w http.ResponseWriter) {
	platformsJoined := strings.ToLower(strings.Join(salesforce.Platforms, ", "))
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text: "Nebo usage:\n" +
			"`/nebo shoes` - find all customers with shoe in the name\n" +
			"`/nebo shopify` - show {" + platformsJoined + "} clients sorted by MRR\n" +
			"`/meet <optional name>` - create a google meet link (this link has to be opened in your searchspring chrome profile or you'll end up in a different meeting :/ )\n" +
			"`/fire` - used when our product is broken and the fire team should assemble immediately to fix it\n" +
			"`/firedown` - used when the fire is out to produce a checklist of tasks that we forget after an intense fire\n" +
			"`/neboidnx` - gets a Nextopia customer ID based on name or id\n" +
			"`/neboidss` - gets a Searchspring customer ID based on name or id\n" +
			"`/nebo help` - this message",
	}
	json, _ := json.Marshal(msg)
	w.Write(json)
}
func writeHelpNeboid(w http.ResponseWriter) {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "Neboid usage:\n`/neboid <id prefix>` - find all customers with an id that starts with this prefix\n`/neboid help` - this message",
	}
	json, _ := json.Marshal(msg)
	w.Write(json)
}

func writeHelpMeet(w http.ResponseWriter) {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeEphemeral,
		Text:         "Meet usage:\n`/meet` - generate a random meet\n`/meet name` - generate a meet with a name\n`/meet help` - this message",
	}
	json, _ := json.Marshal(msg)
	w.Write(json)
}

func meetResponse(search string) []byte {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeInChannel,
		Text:         getMeetLink(search),
	}
	json, _ := json.Marshal(msg)
	return json
}

func getMeetLink(search string) string {
	name := search
	name = strings.ReplaceAll(name, " ", "-")
	if strings.TrimSpace(search) == "" {
		rand.Seed(time.Now().UnixNano())
		name = petname.Generate(3, "-")
	}
	return "g.co/meet/" + name
}

func fireResponse(folderID string, responseURL string) {
	checklist := fireChecklist(folderID)
	postSlackMessage(responseURL, slack.ResponseTypeInChannel, checklist)
}

func postSlackMessage(responseURL string, responseType string, text string) error {
	msg := &slack.Msg{
		ResponseType: responseType,
		Text:         text,
	}
	json, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = http.Post(responseURL, "application/json", bytes.NewBuffer(json))
	return err
}

func fireChecklist(folderID string) string {
	text := "1. Assemble the <!subteam^S01DXD4HKCH> in the <#C01DFMK1F4M> channel\n" +
		"2. Designate fire leader, document maintainer, announcements updater\n" +
		"3. Fire doc maintainer creates a new doc here: " + fmt.Sprintf("<https://drive.google.com/drive/folders/%s>", folderID) + "\n" +
		"4. Post link to the fire doc\n" +
		"5. If a real fire - announcer posts to the <#C024FV14Z> channel \"There is a fire and engineering is investigating, updates will be posted in a thread on this message\"\n" +
		"6. Post a link to the fire document in the <#C024FV14Z> channel thread\n" +
		"7. Fight! " + getMeetLink("fire-investigation-"+timestamp(time.Now())) + "\n\n\n" +
		"8. Use `/firedown` when the fire is out\n"
	return text
}

func fireDownResponse() []byte {
	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeInChannel,
		Text: "1. Ask if there are any cleanup tasks to do\n" +
			"2. Update the <#C024FV14Z>  channel\n" +
			"3. If applicable, schedule a blameless post mortem\n",
	}
	json, _ := json.Marshal(msg)
	return json
}

func timestamp(currentTime time.Time) string {
	return fmt.Sprint(currentTime.UTC().Format("2006-01-02-15-04"))
}

func sendInternalServerError(w http.ResponseWriter, err error) {
	log.Println(err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func findBlankEnvVars(env envVars) []string {
	var blanks []string
	valueOfStruct := reflect.ValueOf(env)
	typeOfStruct := valueOfStruct.Type()
	for i := 0; i < valueOfStruct.NumField(); i++ {
		if valueOfStruct.Field(i).Interface() == "" {
			blanks = append(blanks, typeOfStruct.Field(i).Name)
		}
	}
	return blanks
}
