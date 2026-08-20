CREATE TABLE students (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE scores (
  student_id BIGINT NOT NULL REFERENCES students(id),
  score INTEGER NOT NULL
);
