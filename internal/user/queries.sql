-- name: FindUserByFirebaseUID :one
SELECT * FROM users WHERE firebase_uid = $1;

-- name: FindUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: FindUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: CreateUser :one
INSERT INTO users (firebase_uid, username, display_name, avatar_url)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUserLastSeen :exec
UPDATE users SET last_seen_at = now() WHERE id = $1;

-- name: UpdateUserRating :exec
UPDATE users SET rating = $1 WHERE id = $2;

-- name: SearchUsersByUsername :many
SELECT * FROM users WHERE username LIKE $1 || '%' ORDER BY username LIMIT 10;

-- name: GetUserBestCategory :one
SELECT c.name,
       ROUND(100.0 * SUM(CASE WHEN
           (m.player1_id = @user_id AND mr.p1_correct = true) OR
           (m.player2_id = @user_id AND mr.p2_correct = true)
           THEN 1 ELSE 0 END)::numeric / NULLIF(COUNT(*), 0), 1) as accuracy_pct
FROM match_rounds mr
JOIN matches m ON m.id = mr.match_id
JOIN categories c ON c.id = m.category_id
WHERE (m.player1_id = @user_id OR m.player2_id = @user_id)
  AND m.status = 'completed'
GROUP BY c.id, c.name
ORDER BY accuracy_pct DESC
LIMIT 1;

-- name: CountPendingFriendRequests :one
SELECT COUNT(*) FROM friendships
WHERE addressee_id = $1 AND status = 'pending';
