package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAllQuotaDatesIntersectsEnterpriseUsernameAndTime(t *testing.T) {
	truncateTables(t)
	for _, quotaData := range []*QuotaData{
		{UserID: 1, EnterpriseID: 11, Username: "alice", ModelName: "gpt-a", CreatedAt: 1000, Count: 2, Quota: 100, TokenUsed: 40},
		{UserID: 1, EnterpriseID: 12, Username: "alice", ModelName: "gpt-b", CreatedAt: 1100, Count: 3, Quota: 70, TokenUsed: 30},
		{UserID: 1, EnterpriseID: 11, Username: "alice", ModelName: "gpt-outside-time", CreatedAt: 3000, Count: 9, Quota: 900, TokenUsed: 300},
		{UserID: 2, EnterpriseID: 11, Username: "bob", ModelName: "gpt-c", CreatedAt: 1200, Count: 4, Quota: 80, TokenUsed: 20},
	} {
		require.NoError(t, DB.Create(quotaData).Error)
	}

	rows, err := GetAllQuotaDates(900, 2000, "alice", 11)

	require.NoError(t, err)
	require.Equal(t, []*QuotaData{{
		UserID: 1, Username: "alice", ModelName: "gpt-a", CreatedAt: 1000, Count: 2, Quota: 100, TokenUsed: 40,
	}}, rows)
}
