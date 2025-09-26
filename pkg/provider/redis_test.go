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

func setupTestRedis(t *testing.T, teamID string, userID string) (*RedisClient, redismock.ClientMock, func()) {
	logger := zaptest.NewLogger(t)

	rdb, mock := redismock.NewClientMock()

	client := &RedisClient{
		client: rdb,
		logger: logger,
		teamID: teamID,
		userID: userID,
	}

	cleanup := func() {
		rdb.Close()
	}

	return client, mock, cleanup
}

func TestRedisClient_Users(t *testing.T) {
	teamID := "TEST123"
	userID := "U123456"
	client, mock, cleanup := setupTestRedis(t, teamID, userID)
	defer cleanup()

	ctx := context.Background()

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
	expectedKey := "slack:TEST123/U123456:users"
	mock.ExpectSet(expectedKey, expectedJSON, CacheTTL).SetVal("OK")

	// Test SetUsers
	err = client.SetUsers(ctx, users)
	require.NoError(t, err)

	// Mock GetUsers
	mock.ExpectGet(expectedKey).SetVal(string(expectedJSON))

	// Test GetUsers
	retrievedUsers, err := client.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, users, retrievedUsers)

	// Test GetUsers with non-existent team (Redis returns nil)
	// Note: We'll need a separate client for this test since each client is team/user-scoped
	nonExistentClient, nonExistentMock, nonExistentCleanup := setupTestRedis(t, "NONEXISTENT", "U999999")
	defer nonExistentCleanup()

	nonExistentMock.ExpectGet("slack:NONEXISTENT/U999999:users").RedisNil()
	emptyUsers, err := nonExistentClient.GetUsers(ctx)
	require.NoError(t, err)
	assert.Nil(t, emptyUsers)

	// Check non-existent mock expectations
	err = nonExistentMock.ExpectationsWereMet()
	require.NoError(t, err)

	// Ensure all expectations were met
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestRedisClient_Channels(t *testing.T) {
	teamID := "TEST123"
	userID := "U123456"
	client, mock, cleanup := setupTestRedis(t, teamID, userID)
	defer cleanup()

	ctx := context.Background()

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
	expectedKey := "slack:TEST123/U123456:channels"
	mock.ExpectSet(expectedKey, expectedJSON, CacheTTL).SetVal("OK")

	// Test SetChannels
	err = client.SetChannels(ctx, channels)
	require.NoError(t, err)

	// Mock GetChannels
	mock.ExpectGet(expectedKey).SetVal(string(expectedJSON))

	// Test GetChannels
	retrievedChannels, err := client.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, channels, retrievedChannels)

	// Test GetChannels with non-existent team
	nonExistentClient, nonExistentMock, nonExistentCleanup := setupTestRedis(t, "NONEXISTENT", "U999999")
	defer nonExistentCleanup()

	nonExistentMock.ExpectGet("slack:NONEXISTENT/U999999:channels").RedisNil()
	emptyChannels, err := nonExistentClient.GetChannels(ctx)
	require.NoError(t, err)
	assert.Nil(t, emptyChannels)

	// Check non-existent mock expectations
	err = nonExistentMock.ExpectationsWereMet()
	require.NoError(t, err)

	// Ensure all expectations were met
	err = mock.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestRedisClient_MultiTenant(t *testing.T) {
	ctx := context.Background()
	teamID1 := "TEAM1"
	teamID2 := "TEAM2"
	userID1 := "U111111"
	userID2 := "U222222"

	// Create separate clients for each team/user combination
	client1, mock1, cleanup1 := setupTestRedis(t, teamID1, userID1)
	defer cleanup1()
	client2, mock2, cleanup2 := setupTestRedis(t, teamID2, userID2)
	defer cleanup2()

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

	// Mock SET operations for team 1
	mock1.ExpectSet("slack:TEAM1/U111111:users", users1JSON, CacheTTL).SetVal("OK")
	mock1.ExpectSet("slack:TEAM1/U111111:channels", channels1JSON, CacheTTL).SetVal("OK")

	// Mock SET operations for team 2
	mock2.ExpectSet("slack:TEAM2/U222222:users", users2JSON, CacheTTL).SetVal("OK")
	mock2.ExpectSet("slack:TEAM2/U222222:channels", channels2JSON, CacheTTL).SetVal("OK")

	// Set data for team 1
	err = client1.SetUsers(ctx, users1)
	require.NoError(t, err)
	err = client1.SetChannels(ctx, channels1)
	require.NoError(t, err)

	// Set data for team 2
	err = client2.SetUsers(ctx, users2)
	require.NoError(t, err)
	err = client2.SetChannels(ctx, channels2)
	require.NoError(t, err)

	// Mock GET operations for verification
	mock1.ExpectGet("slack:TEAM1/U111111:users").SetVal(string(users1JSON))
	mock1.ExpectGet("slack:TEAM1/U111111:channels").SetVal(string(channels1JSON))
	mock2.ExpectGet("slack:TEAM2/U222222:users").SetVal(string(users2JSON))
	mock2.ExpectGet("slack:TEAM2/U222222:channels").SetVal(string(channels2JSON))

	// Verify team 1 data
	retrievedUsers1, err := client1.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, users1, retrievedUsers1)

	retrievedChannels1, err := client1.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, channels1, retrievedChannels1)

	// Verify team 2 data
	retrievedUsers2, err := client2.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, users2, retrievedUsers2)

	retrievedChannels2, err := client2.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, channels2, retrievedChannels2)

	// Verify teams/users don't interfere with each other
	assert.NotEqual(t, retrievedUsers1, retrievedUsers2)
	assert.NotEqual(t, retrievedChannels1, retrievedChannels2)

	// Ensure all expectations were met for both mocks
	err = mock1.ExpectationsWereMet()
	require.NoError(t, err)
	err = mock2.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestRedisClient_SameTeamDifferentUsers(t *testing.T) {
	ctx := context.Background()
	teamID := "TEAM123"
	userID1 := "U111111"
	userID2 := "U222222"

	// Create separate clients for different users in the same team
	client1, mock1, cleanup1 := setupTestRedis(t, teamID, userID1)
	defer cleanup1()
	client2, mock2, cleanup2 := setupTestRedis(t, teamID, userID2)
	defer cleanup2()

	// Test data for user 1
	users1 := []slack.User{
		{ID: "U1", Name: "user1"},
	}
	channels1 := []Channel{
		{ID: "C1", Name: "#user1-channel"},
	}

	// Test data for user 2
	users2 := []slack.User{
		{ID: "U2", Name: "user2"},
	}
	channels2 := []Channel{
		{ID: "C2", Name: "#user2-channel"},
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

	// Mock SET operations for user 1
	mock1.ExpectSet("slack:TEAM123/U111111:users", users1JSON, CacheTTL).SetVal("OK")
	mock1.ExpectSet("slack:TEAM123/U111111:channels", channels1JSON, CacheTTL).SetVal("OK")

	// Mock SET operations for user 2
	mock2.ExpectSet("slack:TEAM123/U222222:users", users2JSON, CacheTTL).SetVal("OK")
	mock2.ExpectSet("slack:TEAM123/U222222:channels", channels2JSON, CacheTTL).SetVal("OK")

	// Set data for user 1
	err = client1.SetUsers(ctx, users1)
	require.NoError(t, err)
	err = client1.SetChannels(ctx, channels1)
	require.NoError(t, err)

	// Set data for user 2
	err = client2.SetUsers(ctx, users2)
	require.NoError(t, err)
	err = client2.SetChannels(ctx, channels2)
	require.NoError(t, err)

	// Mock GET operations for verification
	mock1.ExpectGet("slack:TEAM123/U111111:users").SetVal(string(users1JSON))
	mock1.ExpectGet("slack:TEAM123/U111111:channels").SetVal(string(channels1JSON))
	mock2.ExpectGet("slack:TEAM123/U222222:users").SetVal(string(users2JSON))
	mock2.ExpectGet("slack:TEAM123/U222222:channels").SetVal(string(channels2JSON))

	// Verify user 1 data
	retrievedUsers1, err := client1.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, users1, retrievedUsers1)

	retrievedChannels1, err := client1.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, channels1, retrievedChannels1)

	// Verify user 2 data
	retrievedUsers2, err := client2.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, users2, retrievedUsers2)

	retrievedChannels2, err := client2.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, channels2, retrievedChannels2)

	// Verify users don't interfere with each other even in the same team
	assert.NotEqual(t, retrievedUsers1, retrievedUsers2)
	assert.NotEqual(t, retrievedChannels1, retrievedChannels2)

	// Ensure all expectations were met for both mocks
	err = mock1.ExpectationsWereMet()
	require.NoError(t, err)
	err = mock2.ExpectationsWereMet()
	require.NoError(t, err)
}
