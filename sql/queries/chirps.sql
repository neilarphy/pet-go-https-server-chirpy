-- name: CreateChirp :one
INSERT INTO chirps (user_id, body)
VALUES ($1, $2)
RETURNING *;

-- name: GetChirp :one
SELECT * FROM chirps WHERE id = $1;

-- name: ListChirps :many
SELECT * FROM chirps ORDER BY created_at DESC;

-- name: DeleteChirp :exec
DELETE FROM chirps WHERE id = $1;