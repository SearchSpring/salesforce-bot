package salesforce

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nlopes/slack"
	"github.com/simpleforce/simpleforce"
	"github.com/stretchr/testify/require"
)

type salesforceDAOTest struct{}

func createQueryResults() *simpleforce.QueryResult {
	qr := &simpleforce.QueryResult{}
	json.Unmarshal([]byte(`{ "totalSize": 1,
		"done": true,
		"records": [{ 
				"Website": "fabletics.com",
				"CS_Manager__r": { "Name": "Ashley Hilton" },
				"Family_MRR__c": 14858.54,
				"Chargify_MRR__c": 3955.17,
				"Integration_Type__c":"v3",
				"Chargify_Source__c":"Searchspring",
				"Platform__c":"Custom",
				"Tracking_Code__c": "wub9gl"} 
			]
		}`), qr)
	return qr
}

func TestFormatAccountInfos(t *testing.T) {
	dao := &DAOImpl{}
	response, err := dao.ResultToMessage("search term", createQueryResults())
	require.Nil(t, err)
	msg := &slack.Msg{}
	err = json.Unmarshal(response, msg)
	require.Nil(t, err)
	require.True(t, strings.Contains(msg.Text, "search term"))
	require.True(t, strings.Contains(msg.Attachments[0].Text, "Rep: Ashley Hilton"))
	require.True(t, strings.Contains(msg.Attachments[0].Text, "MRR: $3955.17"))
	require.True(t, strings.Contains(msg.Attachments[0].Text, "Platform: Custom"))
	require.True(t, strings.Contains(msg.Attachments[0].Text, "Integration: v3"))
	require.True(t, strings.Contains(msg.Attachments[0].Text, "Provider: Searchspring"))
	require.True(t, strings.Contains(msg.Attachments[0].Text, "Family MRR: $14858.54"))
	require.Equal(t, "fabletics.com (Not active) (SiteId: wub9gl)", msg.Attachments[0].AuthorName)
	require.Equal(t, "#3A23AD", msg.Attachments[0].Color)
}

func c(b []byte, e error) string {
	return string(b)
}
