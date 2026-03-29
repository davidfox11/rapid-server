-- name: ListCategories :many
SELECT * FROM categories ORDER BY name;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1;

-- name: GetMatch :one
SELECT m.*, c.name as category_name,
       u1.username as player1_username, u2.username as player2_username
FROM matches m
JOIN categories c ON c.id = m.category_id
JOIN users u1 ON u1.id = m.player1_id
JOIN users u2 ON u2.id = m.player2_id
WHERE m.id = $1;

-- name: LoadRandomQuestions :many
SELECT * FROM questions
WHERE category_id = $1
ORDER BY random()
LIMIT $2;

-- name: CreateMatch :one
INSERT INTO matches (category_id, player1_id, player2_id, status)
VALUES ($1, $2, $3, 'active')
RETURNING *;

-- name: CompleteMatch :exec
UPDATE matches
SET status = 'completed',
    player1_score = @p1_score,
    player2_score = @p2_score,
    winner_id = @winner_id,
    completed_at = now()
WHERE id = @match_id;

-- name: InsertMatchRound :exec
INSERT INTO match_rounds (match_id, round_number, question_id, p1_choice, p1_correct, p1_time_ms, p1_points, p2_choice, p2_correct, p2_time_ms, p2_points)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetMatchWithRounds :many
SELECT mr.*, q.question_text, q.options, q.correct_index
FROM match_rounds mr
JOIN questions q ON q.id = mr.question_id
WHERE mr.match_id = $1
ORDER BY mr.round_number;

-- name: ListUserMatches :many
SELECT m.*, c.name as category_name,
       u1.username as player1_username, u2.username as player2_username
FROM matches m
JOIN categories c ON c.id = m.category_id
JOIN users u1 ON u1.id = m.player1_id
JOIN users u2 ON u2.id = m.player2_id
WHERE (m.player1_id = @user_id OR m.player2_id = @user_id)
  AND m.status = 'completed'
ORDER BY m.completed_at DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: UpdateBothRatings :exec
UPDATE users SET rating = CASE
    WHEN id = @p1_id THEN @p1_rating::int
    WHEN id = @p2_id THEN @p2_rating::int
END
WHERE id IN (@p1_id, @p2_id);
