package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupTestRedis(t *testing.T) (*RedisClient, redismock.ClientMock, func()) {
	logger := zaptest.NewLogger(t)

	rdb, mock := redismock.NewClientMock()

	client := &RedisClient{
		client: rdb,
		logger: logger,
	}

	cleanup := func() {
		rdb.Close()
	}

	return client, mock, cleanup
}

func TestRedisClient_Users(t *testing.T) {
	client, mock, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	teamID := "TEST123"

	// Test data
	users := []slack.User{
		{
			ID:   "U123",
			Name: "testuser1",
			Profile: slack.UserProfile{
				RealName: "Test User 1",
			},
		},
		{
			ID:   "U456",
			Name: "testuser2",
			Profile: slack.UserProfile{
				RealName: "Test User 2",
			},
		},
	}

	// Generate expected JSON dynamically
	expectedJSON, err := json.Marshal(users)
	require.NoError(t, err)

	// Mock SetUsers
	expectedKey := "slack:TEST123:users"
	mock.ExpectSet(expectedKey, expectedJSON, 0).SetVal("OK")

	// Test SetUsers
	err = client.SetUsers(ctx, teamID, users)
	require.NoError(t, err)

	// Mock GetUsers
	mock.ExpectGet(expectedKey).SetVal(string(expectedJSON))

	// Test GetUsers
	retrievedUsers, err := client.GetUsers(ctx, teamID)
	require.NoError(t, err)
	assert.Equal(t, users, retrievedUsers)

	// Test GetUsers with non-existent team (Redis returns nil)
	mock.ExpectGet("slack:NONEXISTENT:users").RedisNil()
	emptyUsers, err := client.GetUsers(ctx, "NONEXISTENT")
	require.NoError(t, err)
	assert.Nil(t, emptyUsers)

	// Ensure all expectations were met
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestRedisClient_Channels(t *testing.T) {
	client, mock, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	teamID := "TEST123"

	// Test data
	channels := []Channel{
		{
			ID:          "C123",
			Name:        "#general",
			Topic:       "General discussion",
			Purpose:     "Company-wide announcements",
			MemberCount: 100,
			IsIM:        false,
			IsMpIM:      false,
			IsPrivate:   false,
		},
		{
			ID:          "C456",
			Name:        "#random",
			Topic:       "Random chat",
			Purpose:     "Non-work related discussions",
			MemberCount: 50,
			IsIM:        false,
			IsMpIM:      false,
			IsPrivate:   false,
		},
	}

	// Generate expected JSON dynamically
	expectedJSON, err := json.Marshal(channels)
	require.NoError(t, err)

	// Mock SetChannels
	expectedKey := "slack:TEST123:channels"
	mock.ExpectSet(expectedKey, expectedJSON, 0).SetVal("OK")

	// Test SetChannels
	err = client.SetChannels(ctx, teamID, channels)
	require.NoError(t, err)

	// Mock GetChannels
	mock.ExpectGet(expectedKey).SetVal(string(expectedJSON))

	// Test GetChannels
	retrievedChannels, err := client.GetChannels(ctx, teamID)
	require.NoError(t, err)
	assert.Equal(t, channels, retrievedChannels)

	// Test GetChannels with non-existent team
	mock.ExpectGet("slack:NONEXISTENT:channels").RedisNil()
	emptyChannels, err := client.GetChannels(ctx, "NONEXISTENT")
	require.NoError(t, err)
	assert.Nil(t, emptyChannels)

	// Ensure all expectations were met
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestRedisClient_MultiTenant(t *testing.T) {
	client, mock, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	teamID1 := "TEAM1"
	teamID2 := "TEAM2"

	// Test data for team 1
	users1 := []slack.User{
		{ID: "U1", Name: "user1"},
	}
	channels1 := []Channel{
		{ID: "C1", Name: "#team1-general"},
	}

	// Test data for team 2
	users2 := []slack.User{
		{ID: "U2", Name: "user2"},
	}
	channels2 := []Channel{
		{ID: "C2", Name: "#team2-general"},
	}

	// Generate expected JSON dynamically
	users1JSON, err := json.Marshal(users1)
	require.NoError(t, err)
	channels1JSON, err := json.Marshal(channels1)
	require.NoError(t, err)
	users2JSON, err := json.Marshal(users2)
	require.NoError(t, err)
	channels2JSON, err := json.Marshal(channels2)
	require.NoError(t, err)

	// Mock SET operations for both teams
	mock.ExpectSet("slack:TEAM1:users", users1JSON, 0).SetVal("OK")
	mock.ExpectSet("slack:TEAM1:channels", channels1JSON, 0).SetVal("OK")
	mock.ExpectSet("slack:TEAM2:users", users2JSON, 0).SetVal("OK")
	mock.ExpectSet("slack:TEAM2:channels", channels2JSON, 0).SetVal("OK")

	// Set data for both teams
	err = client.SetUsers(ctx, teamID1, users1)
	require.NoError(t, err)
	err = client.SetChannels(ctx, teamID1, channels1)
	require.NoError(t, err)

	err = client.SetUsers(ctx, teamID2, users2)
	require.NoError(t, err)
	err = client.SetChannels(ctx, teamID2, channels2)
	require.NoError(t, err)

	// Mock GET operations for verification
	mock.ExpectGet("slack:TEAM1:users").SetVal(string(users1JSON))
	mock.ExpectGet("slack:TEAM1:channels").SetVal(string(channels1JSON))
	mock.ExpectGet("slack:TEAM2:users").SetVal(string(users2JSON))
	mock.ExpectGet("slack:TEAM2:channels").SetVal(string(channels2JSON))

	// Verify team 1 data
	retrievedUsers1, err := client.GetUsers(ctx, teamID1)
	require.NoError(t, err)
	assert.Equal(t, users1, retrievedUsers1)

	retrievedChannels1, err := client.GetChannels(ctx, teamID1)
	require.NoError(t, err)
	assert.Equal(t, channels1, retrievedChannels1)

	// Verify team 2 data
	retrievedUsers2, err := client.GetUsers(ctx, teamID2)
	require.NoError(t, err)
	assert.Equal(t, users2, retrievedUsers2)

	retrievedChannels2, err := client.GetChannels(ctx, teamID2)
	require.NoError(t, err)
	assert.Equal(t, channels2, retrievedChannels2)

	// Verify teams don't interfere with each other
	assert.NotEqual(t, retrievedUsers1, retrievedUsers2)
	assert.NotEqual(t, retrievedChannels1, retrievedChannels2)

	// Ensure all expectations were met
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}