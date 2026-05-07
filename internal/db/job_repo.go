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
		`INSERT INTO jobs(id,status,input_path,created_at) VALUES(?,?,?,?)`,
		j.ID, j.Status, j.InputPath, j.CreatedAt)
	return err
}

// GetJobByID busca um job pelo ID
func GetJobByID(ctx context.Context, conn *sql.DB, id string) (Job, error) {
	var j Job
	row := conn.QueryRowContext(ctx,
		`SELECT id,status,input_path,output_dir,created_at,updated_at FROM jobs WHERE id = ?`, id)
	err := row.Scan(&j.ID, &j.Status, &j.InputPath, &j.OutputDir, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

func UpdateJobStatus(conn *sql.DB, id, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		`UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func UpdateJobOutputDir(conn *sql.DB, id, outputDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		`UPDATE jobs SET output_dir = ?, updated_at = ? WHERE id = ?`,
		outputDir, time.Now(), id,
	)
	return err
}


func DeleteJob(conn *sql.DB, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		`DELETE FROM jobs WHERE id = ?`, id)
	return err
}