package db

import (
	"time"
)

type Participant struct {
	ID   string `bson:"_id"`
	Name string `bson:"name"`
}

type Tournament struct {
	ID               int               `bson:"id"`
	Name             string            `bson:"name"`
	Participants     []string          `bson:"participants"`
	MinParticipants  int               `bson:"min_participants"`
	MaxParticipants  int               `bson:"max_participants"`
	TeamCategory     string            `bson:"team_category"`
	ParticipantTeams map[string]string `bson:"participant_teams"`
	Matches          []Match           `bson:"matches"`
	Standings        []Standing        `bson:"standings"`
	IsActive         bool              `bson:"is_active"`
	SetupCompleted   bool              `bson:"setup_completed"`
	CreatedAt        time.Time         `bson:"created_at"`
	Playoff          *Playoff          `bson:"playoff,omitempty"`
}

type Playoff struct {
	CurrentStage  string  `bson:"current_stage"`
	QuarterFinals []Match `bson:"quarter_finals"`
	SemiFinals    []Match `bson:"semi_finals"`
	Final         *Match  `bson:"final"`
	Winner        string  `bson:"winner"`
}

type TeamCategory struct {
	Name  string   `bson:"name"`
	Teams []string `bson:"teams"`
}

type Match struct {
	Team1   string    `bson:"team1"`
	Team2   string    `bson:"team2"`
	Score1  int       `bson:"score1"`
	Score2  int       `bson:"score2"`
	Date    time.Time `bson:"date"`
	Counted bool      `bson:"counted"`
}

type Standing struct {
	Team            string `bson:"team"`
	Played          int    `bson:"played"`
	Won             int    `bson:"won"`
	Drawn           int    `bson:"drawn"`
	Lost            int    `bson:"lost"`
	GoalsFor        int    `bson:"goals_for"`
	GoalsAgainst    int    `bson:"goals_against"`
	GoalsDifference int    `bson:"goals_difference"`
	Points          int    `bson:"points"`
}

func (t *Tournament) HasParticipant(participantName string) bool {
	for _, p := range t.Participants {
		if p == participantName {
			return true
		}
	}
	return false
}

type Admin struct {
	UserID int64 `bson:"user_id"`
}
