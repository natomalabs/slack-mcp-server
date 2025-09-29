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

func setupTestRedis(t *testing.T, instanceID string, userID string) (*RedisClient, redismock.ClientMock, func()) {
	logger := zaptest.NewLogger(t)

	rdb, mock := redismock.NewClientMock()

	client := &RedisClient{
		client:     rdb,
		logger:     logger,
		instanceID: instanceID,
		userID:     userID,
	}

	cleanup := func() {
		rdb.Close()
	}

	return client, mock, cleanup
}

func TestRedisClient_Users(t *testing.T) {
	instanceID := "TEST123"
	userID := "U123456"
	client, mock, cleanup := setupTestRedis(t, instanceID, userID)
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

	// Test GetUsers with non-existent instance (Redis returns nil)
	// Note: We'll need a separate client for this test since each client is instance/user-scoped
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
	instanceID := "TEST123"
	userID := "U123456"
	client, mock, cleanup := setupTestRedis(t, instanceID, userID)
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

	// Test GetChannels with non-existent instance
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
	instanceID1 := "TEAM1"
	instanceID2 := "TEAM2"
	userID1 := "U111111"
	userID2 := "U222222"

	// Create separate clients for each instance/user combination
	client1, mock1, cleanup1 := setupTestRedis(t, instanceID1, userID1)
	defer cleanup1()
	client2, mock2, cleanup2 := setupTestRedis(t, instanceID2, userID2)
	defer cleanup2()

	// Test data for instance 1
	users1 := []slack.User{
		{ID: "U1", Name: "user1"},
	}
	channels1 := []Channel{
		{ID: "C1", Name: "#team1-general"},
	}

	// Test data for instance 2
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

	// Mock SET operations for instance 1
	mock1.ExpectSet("slack:TEAM1/U111111:users", users1JSON, CacheTTL).SetVal("OK")
	mock1.ExpectSet("slack:TEAM1/U111111:channels", channels1JSON, CacheTTL).SetVal("OK")

	// Mock SET operations for instance 2
	mock2.ExpectSet("slack:TEAM2/U222222:users", users2JSON, CacheTTL).SetVal("OK")
	mock2.ExpectSet("slack:TEAM2/U222222:channels", channels2JSON, CacheTTL).SetVal("OK")

	// Set data for instance 1
	err = client1.SetUsers(ctx, users1)
	require.NoError(t, err)
	err = client1.SetChannels(ctx, channels1)
	require.NoError(t, err)

	// Set data for instance 2
	err = client2.SetUsers(ctx, users2)
	require.NoError(t, err)
	err = client2.SetChannels(ctx, channels2)
	require.NoError(t, err)

	// Mock GET operations for verification
	mock1.ExpectGet("slack:TEAM1/U111111:users").SetVal(string(users1JSON))
	mock1.ExpectGet("slack:TEAM1/U111111:channels").SetVal(string(channels1JSON))
	mock2.ExpectGet("slack:TEAM2/U222222:users").SetVal(string(users2JSON))
	mock2.ExpectGet("slack:TEAM2/U222222:channels").SetVal(string(channels2JSON))

	// Verify instance 1 data
	retrievedUsers1, err := client1.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, users1, retrievedUsers1)

	retrievedChannels1, err := client1.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, channels1, retrievedChannels1)

	// Verify instance 2 data
	retrievedUsers2, err := client2.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, users2, retrievedUsers2)

	retrievedChannels2, err := client2.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, channels2, retrievedChannels2)

	// Verify instances/users don't interfere with each other
	assert.NotEqual(t, retrievedUsers1, retrievedUsers2)
	assert.NotEqual(t, retrievedChannels1, retrievedChannels2)

	// Ensure all expectations were met for both mocks
	err = mock1.ExpectationsWereMet()
	require.NoError(t, err)
	err = mock2.ExpectationsWereMet()
	require.NoError(t, err)
}

func TestRedisClient_SameInstanceDifferentUsers(t *testing.T) {
	ctx := context.Background()
	instanceID := "TEAM123"
	userID1 := "U111111"
	userID2 := "U222222"

	// Create separate clients for different users in the same instance
	client1, mock1, cleanup1 := setupTestRedis(t, instanceID, userID1)
	defer cleanup1()
	client2, mock2, cleanup2 := setupTestRedis(t, instanceID, userID2)
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

	// Verify users don't interfere with each other even in the same instance
	assert.NotEqual(t, retrievedUsers1, retrievedUsers2)
	assert.NotEqual(t, retrievedChannels1, retrievedChannels2)

	// Ensure all expectations were met for both mocks
	err = mock1.ExpectationsWereMet()
	require.NoError(t, err)
	err = mock2.ExpectationsWereMet()
	require.NoError(t, err)
}

// TestRedisClient_EnterpriseWorkspace tests that enterprise workspaces use enterpriseID as instanceID
// and are properly separated from non-enterprise workspaces
func TestRedisClient_EnterpriseWorkspace(t *testing.T) {
	ctx := context.Background()

	// Enterprise workspace (uses enterpriseID as instanceID)
	enterpriseID := "E0160NTJ2PM"
	enterpriseUserID := "U1234567890"
	enterpriseClient, enterpriseMock, enterpriseCleanup := setupTestRedis(t, enterpriseID, enterpriseUserID)
	defer enterpriseCleanup()

	// Non-enterprise workspace (uses teamID as instanceID)
	teamID := "TEAM123"
	teamUserID := "U9876543210"
	teamClient, teamMock, teamCleanup := setupTestRedis(t, teamID, teamUserID)
	defer teamCleanup()

	// Test data for enterprise workspace
	enterpriseUsers := []slack.User{
		{ID: "U1", Name: "enterprise_user1"},
	}
	enterpriseChannels := []Channel{
		{ID: "C1", Name: "#enterprise-general"},
	}

	// Test data for non-enterprise workspace
	teamUsers := []slack.User{
		{ID: "U2", Name: "team_user1"},
	}
	teamChannels := []Channel{
		{ID: "C2", Name: "#team-general"},
	}

	// Generate expected JSON dynamically
	enterpriseUsersJSON, err := json.Marshal(enterpriseUsers)
	require.NoError(t, err)
	enterpriseChannelsJSON, err := json.Marshal(enterpriseChannels)
	require.NoError(t, err)
	teamUsersJSON, err := json.Marshal(teamUsers)
	require.NoError(t, err)
	teamChannelsJSON, err := json.Marshal(teamChannels)
	require.NoError(t, err)

	// Mock SET operations for enterprise workspace
	enterpriseMock.ExpectSet("slack:E0160NTJ2PM/U1234567890:users", enterpriseUsersJSON, CacheTTL).SetVal("OK")
	enterpriseMock.ExpectSet("slack:E0160NTJ2PM/U1234567890:channels", enterpriseChannelsJSON, CacheTTL).SetVal("OK")

	// Mock SET operations for non-enterprise workspace
	teamMock.ExpectSet("slack:TEAM123/U9876543210:users", teamUsersJSON, CacheTTL).SetVal("OK")
	teamMock.ExpectSet("slack:TEAM123/U9876543210:channels", teamChannelsJSON, CacheTTL).SetVal("OK")

	// Set data for enterprise workspace
	err = enterpriseClient.SetUsers(ctx, enterpriseUsers)
	require.NoError(t, err)
	err = enterpriseClient.SetChannels(ctx, enterpriseChannels)
	require.NoError(t, err)

	// Set data for non-enterprise workspace
	err = teamClient.SetUsers(ctx, teamUsers)
	require.NoError(t, err)
	err = teamClient.SetChannels(ctx, teamChannels)
	require.NoError(t, err)

	// Mock GET operations for verification
	enterpriseMock.ExpectGet("slack:E0160NTJ2PM/U1234567890:users").SetVal(string(enterpriseUsersJSON))
	enterpriseMock.ExpectGet("slack:E0160NTJ2PM/U1234567890:channels").SetVal(string(enterpriseChannelsJSON))
	teamMock.ExpectGet("slack:TEAM123/U9876543210:users").SetVal(string(teamUsersJSON))
	teamMock.ExpectGet("slack:TEAM123/U9876543210:channels").SetVal(string(teamChannelsJSON))

	// Verify enterprise workspace data
	retrievedEnterpriseUsers, err := enterpriseClient.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, enterpriseUsers, retrievedEnterpriseUsers)

	retrievedEnterpriseChannels, err := enterpriseClient.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, enterpriseChannels, retrievedEnterpriseChannels)

	// Verify non-enterprise workspace data
	retrievedTeamUsers, err := teamClient.GetUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, teamUsers, retrievedTeamUsers)

	retrievedTeamChannels, err := teamClient.GetChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, teamChannels, retrievedTeamChannels)

	// Verify enterprise and non-enterprise workspaces don't interfere with each other
	assert.NotEqual(t, retrievedEnterpriseUsers, retrievedTeamUsers)
	assert.NotEqual(t, retrievedEnterpriseChannels, retrievedTeamChannels)

	// Ensure all expectations were met for both mocks
	err = enterpriseMock.ExpectationsWereMet()
	require.NoError(t, err)
	err = teamMock.ExpectationsWereMet()
	require.NoError(t, err)
}
