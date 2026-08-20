-- name: GetStudentScore :one
SELECT sqlc.embed(students), sqlc.embed(scores)
FROM students
JOIN scores ON scores.student_id = students.id
WHERE students.id = $1;
