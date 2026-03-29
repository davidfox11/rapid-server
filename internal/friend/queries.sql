-- name: CreateFriendship :one
INSERT INTO friendships (requester_id, addressee_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateFriendshipStatus :exec
UPDATE friendships SET status = $1 WHERE id = $2;

-- name: GetFriendship :one
SELECT * FROM friendships WHERE id = $1;

-- name: FindFriendshipBetween :one
SELECT * FROM friendships
WHERE (requester_id = @user_a AND addressee_id = @user_b)
   OR (requester_id = @user_b AND addressee_id = @user_a);

-- name: ListAcceptedFriends :many
SELECT u.* FROM users u
JOIN friendships f ON (
    (f.requester_id = @user_id AND f.addressee_id = u.id) OR
    (f.addressee_id = @user_id AND f.requester_id = u.id)
)
WHERE f.status = 'accepted';

-- name: ListPendingFriendRequests :many
SELECT f.*, u.username, u.display_name, u.avatar_url, u.default_avatar_index, u.rating
FROM friendships f
JOIN users u ON u.id = f.requester_id
WHERE f.addressee_id = $1 AND f.status = 'pending';

-- name: GetH2HRecord :one
SELECT
    COUNT(*) FILTER (WHERE winner_id = @user_id) as wins,
    COUNT(*) FILTER (WHERE winner_id = @opponent_id) as losses
FROM matches
WHERE status = 'completed'
  AND ((player1_id = @user_id AND player2_id = @opponent_id)
    OR (player1_id = @opponent_id AND player2_id = @user_id));
