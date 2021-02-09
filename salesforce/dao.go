package salesforce

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/nlopes/slack"
	"github.com/searchspring/nebo/validator"
	"github.com/simpleforce/simpleforce"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Platforms is a list of platforms in salesforce
var Platforms = []string{
	"3dcart",
	"BigCommerce",
	"CommerceV3",
	"Custom",
	"Magento",
	"Miva",
	"Netsuite",
	"Other",
	"Shopify",
	"Shopify Plus",
	"Yahoo",
}

type accountInfo struct {
	Website     string
	Manager     string
	Active      string
	MRR         float64
	FamilyMRR   float64
	Platform    string
	Integration string
	Provider    string
	SiteId      string
}

// DAO acts as the salesforce DAO
type DAO interface {
	Query(query string) ([]byte, error)
	IDQuery(query string) ([]byte, error)
	ResultToMessage(query string, result *simpleforce.QueryResult) ([]byte, error)
}

// DAOImpl defines the properties of the DAO
type DAOImpl struct {
	Client *simpleforce.Client
}

// NewDAO returns the salesforce DAO
func NewDAO(sfURL string, sfUser string, sfPassword string, sfToken string) DAO {
	if validator.ContainsEmptyString(sfURL, sfUser, sfPassword, sfToken) {
		return nil
	}
	client := simpleforce.NewClient(sfURL, simpleforce.DefaultClientID, simpleforce.DefaultAPIVersion)
	if client == nil {
		log.Println("nil returned from client creation")
		return nil
	}
	err := client.LoginPassword(sfUser, sfPassword, sfToken)
	if err != nil {
		log.Println(err.Error())
		return nil
	}
	return &DAOImpl{
		Client: client,
	}
}

func (s *DAOImpl) Query(search string) ([]byte, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9_.-]+")
	if err != nil {
		return nil, err
	}

	sanitized := reg.ReplaceAllString(search, "")

	q := "SELECT Type, Website, CS_Manager__r.Name, Family_MRR__c, Chargify_MRR__c, Platform__c, Integration_Type__c, Chargify_Source__c, Tracking_Code__c " +
		"FROM Account WHERE Type IN ('Customer', 'Inactive Customer') " +
		"AND (Website LIKE '%" + sanitized + "%' OR Platform__c LIKE '%" + sanitized +
		"%' OR Tracking_Code__c = '" + sanitized + "') ORDER BY Chargify_MRR__c DESC"
	result, err := s.Client.Query(q)
	if err != nil {
		return nil, err
	}
	return s.ResultToMessage(sanitized, result)
}

func (s *DAOImpl) IDQuery(search string) ([]byte, error) {
	reg, err := regexp.Compile("[^a-zA-Z0-9_.-]+")
	if err != nil {
		return nil, err
	}

	sanitized := reg.ReplaceAllString(search, "")

	q := "SELECT Type, Website, CS_Manager__r.Name, Family_MRR__c, Chargify_MRR__c, Platform__c, Integration_Type__c, Chargify_Source__c, Tracking_Code__c " +
		"FROM Account WHERE Type IN ('Customer', 'Inactive Customer') AND Tracking_Code__c = '" + sanitized + "' ORDER BY Chargify_MRR__c DESC"
	result, err := s.Client.Query(q)
	if err != nil {
		return nil, err
	}
	return s.ResultToMessage(sanitized, result)
}

func (s *DAOImpl) ResultToMessage(search string, result *simpleforce.QueryResult) ([]byte, error) {
	accounts := []*accountInfo{}
	for _, record := range result.Records {
		manager := record["CS_Manager__r"]
		managerName := "unknown"
		if manager != nil {
			if mapName, ok := (manager.(map[string]interface{}))["Name"]; ok {
				managerName = fmt.Sprintf("%s", mapName)
			}
		}
		Type := record["Type"]
		active := "Active"
		if Type != "Customer" {
			active = "Not active"
		}
		platform := "unknown"
		if record["Platform__c"] != nil {
			platform = fmt.Sprintf("%s", record["Platform__c"])
		}
		integration := "unknown"
		if record["Integration_Type__c"] != nil {
			integration = fmt.Sprintf("%s", record["Integration_Type__c"])
		}
		provider := "unknown"
		if record["Chargify_Source__c"] != nil {
			provider = fmt.Sprintf("%s", record["Chargify_Source__c"])
		}
		mrr := float64(-1)
		if record["Chargify_MRR__c"] != nil {
			mrr = record["Chargify_MRR__c"].(float64)
		}
		familymrr := float64(-1)
		if record["Family_MRR__c"] != nil {
			familymrr = record["Family_MRR__c"].(float64)
		}
		siteId := "unknown"
		if record["Tracking_Code__c"] != nil {
			siteId = fmt.Sprintf("%s", record["Tracking_Code__c"])
		}

		accounts = append(accounts, &accountInfo{
			Website:     fmt.Sprintf("%s", record["Website"]),
			Manager:     fmt.Sprintf("%s", managerName),
			Active:      fmt.Sprintf("%s", active),
			MRR:         mrr,
			FamilyMRR:   familymrr,
			Platform:    platform,
			Integration: integration,
			Provider:    provider,
			SiteId:      siteId,
		})
	}
	accounts = cleanAccounts(accounts)
	if !isPlatformSearch(search) {
		accounts = sortAccounts(accounts)
	}
	accounts = truncateAccounts(accounts)
	msg := formatAccountInfos(accounts, search)
	return json.Marshal(msg)
}

func truncateAccounts(accounts []*accountInfo) []*accountInfo {
	truncated := []*accountInfo{}
	for i, account := range accounts {
		if i == 20 {
			break
		}
		truncated = append(truncated, account)
	}
	return truncated
}
func isPlatformSearch(search string) bool {
	for _, platform := range Platforms {
		if strings.ToLower(search) == strings.ToLower(platform) {
			return true
		}
	}
	return false
}

// example formatting here: https://api.slack.com/reference/messaging/attachments
func formatAccountInfos(accountInfos []*accountInfo, search string) *slack.Msg {
	initialText := "Reps for search: " + search
	if len(accountInfos) == 0 {
		initialText = "No results for: " + search
	}

	p := message.NewPrinter(language.English)

	msg := &slack.Msg{
		ResponseType: slack.ResponseTypeInChannel,
		Text:         initialText,
		Attachments:  []slack.Attachment{},
	}
	for _, ai := range accountInfos {
		color := "3A23AD" // Searchspring purple
		if ai.Manager == "unknown" {
			color = "FF0000" // red
		}
		mrr := "unknown"
		if ai.MRR != -1 {
			mrr = p.Sprintf("$%.2f", ai.MRR)
		}
		familymrr := "unknown"
		if ai.FamilyMRR != -1 {
			familymrr = p.Sprintf("$%.2f", ai.FamilyMRR)
		}
		mrr = mrr + " (Family MRR: " + familymrr + ")"
		text := "Rep: " + ai.Manager + "\nMRR: " + mrr + "\nPlatform: " + ai.Platform + "\nIntegration: " + ai.Integration + "\nProvider: " + ai.Provider
		msg.Attachments = append(msg.Attachments, slack.Attachment{
			Color:      "#" + color,
			Text:       text,
			AuthorName: ai.Website + " (" + ai.Active + ") (SiteId: " + ai.SiteId + ")",
		})
	}
	return msg
}

func cleanAccounts(accounts []*accountInfo) []*accountInfo {
	for _, account := range accounts {
		w := account.Website
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			w = w[strings.Index(w, ":")+3:]
		}
		if strings.HasPrefix(w, "www.") {
			w = w[4:]
		}
		if strings.HasSuffix(w, "/") {
			w = w[0 : len(w)-1]
		}
		account.Website = w
	}
	return accounts
}
func sortAccounts(accounts []*accountInfo) []*accountInfo {
	sort.Slice(accounts, func(i, j int) bool {
		return len(accounts[i].Website) < len(accounts[j].Website)
	})
	return accounts
}
