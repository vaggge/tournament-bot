db.getSiblingDB('tournament').createCollection("admins", {
    validator: {
        $jsonSchema: {
            bsonType: "object",
            required: ["user_id"],
            properties: {
                user_id: {
                    bsonType: "long"
                }
            }
        }
    }
});
