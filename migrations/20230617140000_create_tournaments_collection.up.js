db.getSiblingDB('tournament').createCollection("tournaments", {
    validator: {
        $jsonSchema: {
            bsonType: "object",
            required: [
                "id",
                "name",
                "participants",
                "min_participants",
                "max_participants",
                "team_category",
                "participant_teams",
                "matches",
                "standings",
                "is_active",
                "setup_completed",
                "created_at",
                "is_completed"
            ],
            properties: {
                id: { bsonType: "int" },
                name: { bsonType: "string" },
                participants: {
                    bsonType: "array",
                    items: { bsonType: "string" }
                },
                min_participants: { bsonType: "int" },
                max_participants: { bsonType: "int" },
                team_category: { bsonType: "string" },
                participant_teams: { bsonType: "object" },
                matches: {
                    bsonType: "array",
                    items: {
                        bsonType: "object",
                        required: ["team1", "team2", "score1", "score2", "extra_time", "penalties", "date", "counted"],
                        properties: {
                            team1: { bsonType: "string" },
                            team2: { bsonType: "string" },
                            score1: { bsonType: "int" },
                            score2: { bsonType: "int" },
                            extra_time: { bsonType: "bool" },
                            penalties: { bsonType: "bool" },
                            date: { bsonType: "date" },
                            counted: { bsonType: "bool" }
                        }
                    }
                },
                standings: {
                    bsonType: "array",
                    items: {
                        bsonType: "object",
                        required: ["team", "played", "won", "drawn", "lost", "goals_for", "goals_against", "goals_difference", "points"],
                        properties: {
                            team: { bsonType: "string" },
                            played: { bsonType: "int" },
                            won: { bsonType: "int" },
                            drawn: { bsonType: "int" },
                            lost: { bsonType: "int" },
                            goals_for: { bsonType: "int" },
                            goals_against: { bsonType: "int" },
                            goals_difference: { bsonType: "int" },
                            points: { bsonType: "int" }
                        }
                    }
                },
                is_active: { bsonType: "bool" },
                setup_completed: { bsonType: "bool" },
                created_at: { bsonType: "date" },
                is_completed: { bsonType: "bool" }
            }
        }
    }
});
db.getSiblingDB('tournament').runCommand({
    collMod: "tournaments",
    validator: {
        $jsonSchema: {
            bsonType: "object",
            required: ["playoff"],
            properties: {
                playoff: {
                    bsonType: "object",
                    required: ["current_stage", "quarter_finals", "semi_finals", "final", "winner"],
                    properties: {
                        current_stage: { bsonType: "string" },
                        quarter_finals: {
                            bsonType: "array",
                            items: {
                                bsonType: "object",
                                required: ["team1", "team2", "score1", "score2", "extra_time", "penalties", "date", "counted"],
                                properties: {
                                    team1: { bsonType: "string" },
                                    team2: { bsonType: "string" },
                                    score1: { bsonType: "int" },
                                    score2: { bsonType: "int" },
                                    extra_time: { bsonType: "bool" },
                                    penalties: { bsonType: "bool" },
                                    date: { bsonType: "date" },
                                    counted: { bsonType: "bool" }
                                }
                            }
                        },
                        semi_finals: {
                            bsonType: "array",
                            items: {
                                bsonType: "object",
                                required: ["team1", "team2", "score1", "score2", "extra_time", "penalties", "date", "counted"],
                                properties: {
                                    team1: { bsonType: "string" },
                                    team2: { bsonType: "string" },
                                    score1: { bsonType: "int" },
                                    score2: { bsonType: "int" },
                                    extra_time: { bsonType: "bool" },
                                    penalties: { bsonType: "bool" },
                                    date: { bsonType: "date" },
                                    counted: { bsonType: "bool" }
                                }
                            }
                        },
                        final: {
                            bsonType: "object",
                            required: ["team1", "team2", "score1", "score2", "extra_time", "penalties", "date", "counted"],
                            properties: {
                                team1: { bsonType: "string" },
                                team2: { bsonType: "string" },
                                score1: { bsonType: "int" },
                                score2: { bsonType: "int" },
                                extra_time: { bsonType: "bool" },
                                penalties: { bsonType: "bool" },
                                date: { bsonType: "date" },
                                counted: { bsonType: "bool" }
                            }
                        },
                        winner: { bsonType: "string" }
                    }
                }
            }
        }
    },
    validationLevel: "moderate"
});
