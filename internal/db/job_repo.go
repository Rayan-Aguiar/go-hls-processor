package db

import (
	"context"
	"database/sql"
	"time"
)

type Job struct {
	ID        string
	Status    string
	InputPath string
	OutputDir sql.NullString
	CreatedAt time.Time
	UpdatedAt sql.NullTime
}

// InsertJob insere um novo job no banco de dados
func InsertJob(conn *sql.DB, j Job) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		`INSERT INTO jobs(id,status,input_path,created_at) VALUES($1,$2,$3,$4)`,
		j.ID, j.Status, j.InputPath, j.CreatedAt)
	return err
}

// GetJobByID busca um job pelo ID
func GetJobByID(ctx context.Context, conn *sql.DB, id string) (Job, error) {
	var j Job
	row := conn.QueryRowContext(ctx,
		`SELECT id,status,input_path,output_dir,created_at,updated_at FROM jobs WHERE id = $1`, id)
	err := row.Scan(&j.ID, &j.Status, &j.InputPath, &j.OutputDir, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func UpdateJobStatus(conn *sql.DB, id, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		`UPDATE jobs SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now(), id,
	)
	return err
}

func UpdateJobOutputDir(conn *sql.DB, id, outputDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		`UPDATE jobs SET output_dir = $1, updated_at = $2 WHERE id = $3`,
		outputDir, time.Now(), id,
	)
	return err
}

func DeleteJob(conn *sql.DB, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		`DELETE FROM jobs WHERE id = $1`, id)
	return err
}

func ListStuckProcessingJobs(conn *sql.Conn, cutoff time.Time, limit int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 100
	}

	rows, err := conn.QueryContext(ctx, `
        SELECT id
        FROM jobs
        WHERE status = $1
          AND COALESCE(updated_at, created_at) < $2
        ORDER BY created_at ASC
        LIMIT $3
    `, "processing", cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}
