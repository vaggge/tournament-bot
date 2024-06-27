db.getSiblingDB('tournament').createCollection("team_categories", {
    validator: {
        $jsonSchema: {
            bsonType: "object",
            required: ["name", "teams"],
            properties: {
                name: {
                    bsonType: "string"
                },
                teams: {
                    bsonType: "array",
                    items: {
                        bsonType: "string"
                    }
                }
            }
        }
    }
});
