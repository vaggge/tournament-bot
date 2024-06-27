db.getSiblingDB('tournament').createCollection("participants", {
    validator: {
        $jsonSchema: {
            bsonType: "object",
            required: ["name", "stats"],
            properties: {
                name: {
                    bsonType: "string"
                },
                stats: {
                    bsonType: "object",
                    required: [
                        "total_points",
                        "goals_scored",
                        "goals_conceded",
                        "wins",
                        "losses",
                        "draws",
                        "matches_played",
                        "tournaments_played",
                        "tournament_stats"
                    ],
                    properties: {
                        total_points: {
                            bsonType: "int"
                        },
                        goals_scored: {
                            bsonType: "int"
                        },
                        goals_conceded: {
                            bsonType: "int"
                        },
                        wins: {
                            bsonType: "int"
                        },
                        losses: {
                            bsonType: "int"
                        },
                        draws: {
                            bsonType: "int"
                        },
                        matches_played: {
                            bsonType: "int"
                        },
                        tournaments_played: {
                            bsonType: "int"
                        },
                        tournament_stats: {
                            bsonType: "array",
                            items: {
                                bsonType: "object",
                                required: [
                                    "tournament_id",
                                    "place",
                                    "points",
                                    "goals_scored",
                                    "goals_conceded",
                                    "wins",
                                    "losses",
                                    "draws",
                                    "matches_played"
                                ],
                                properties: {
                                    tournament_id: {
                                        bsonType: "int"
                                    },
                                    place: {
                                        bsonType: "string"
                                    },
                                    points: {
                                        bsonType: "int"
                                    },
                                    goals_scored: {
                                        bsonType: "int"
                                    },
                                    goals_conceded: {
                                        bsonType: "int"
                                    },
                                    wins: {
                                        bsonType: "int"
                                    },
                                    losses: {
                                        bsonType: "int"
                                    },
                                    draws: {
                                        bsonType: "int"
                                    },
                                    matches_played: {
                                        bsonType: "int"
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
});
