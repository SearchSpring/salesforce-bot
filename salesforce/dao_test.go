package salesforce

import (
	"encoding/json"
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
                "Tracking_Code__c": "wub9gl",
                "BillingCity": "Chicago",
                "BillingState": "IL"} 
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
	require.Contains(t, msg.Text, "search term")
	require.Contains(t, msg.Attachments[0].Text, "Rep: Ashley Hilton")
	require.Contains(t, msg.Attachments[0].Text, "MRR: $3,955.17")
	require.Contains(t, msg.Attachments[0].Text, "Platform: Custom")
	require.Contains(t, msg.Attachments[0].Text, "Integration: v3")
	require.Contains(t, msg.Attachments[0].Text, "Provider: Searchspring")
	require.Contains(t, msg.Attachments[0].Text, "Family MRR: $14,858.54")
	require.Contains(t, msg.Attachments[0].Text, "Location: Chicago, IL")
	require.Equal(t, "fabletics.com (Not active) (SiteId: wub9gl)", msg.Attachments[0].AuthorName)
	require.Equal(t, "#3A23AD", msg.Attachments[0].Color)
}

func c(b []byte, e error) string {
	return string(b)
}
