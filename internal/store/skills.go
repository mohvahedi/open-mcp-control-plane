package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

func (r *SQLiteRepository) CreateSkill(ctx context.Context, skill domain.Skill) (domain.Skill, error) {
	now := time.Now().UTC()
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = now
	}
	skill.UpdatedAt = now
	if skill.Tags == nil {
		skill.Tags = []string{}
	}
	if skill.Tools == nil {
		skill.Tools = []domain.ToolMetadata{}
	}
	if skill.Requires == nil {
		skill.Requires = []string{}
	}
	if skill.Provenance == nil {
		skill.Provenance = map[string]any{}
	}
	if skill.Labels == nil {
		skill.Labels = map[string]string{}
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO skills (
		id,name,description,version,source,kind,license,tags,entry_point,content,tools,requires,provenance,labels,created_at,updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		skill.ID, skill.Name, skill.Description, skill.Version, skill.Source, skill.Kind, skill.License,
		mustJSON(skill.Tags), skill.EntryPoint, skill.Content, mustJSON(skill.Tools), mustJSON(skill.Requires),
		mustJSON(skill.Provenance), mustJSON(skill.Labels),
		skill.CreatedAt.Format(time.RFC3339Nano), skill.UpdatedAt.Format(time.RFC3339Nano),
	)
	return skill, err
}

func (r *SQLiteRepository) GetSkill(ctx context.Context, id string) (domain.Skill, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,name,description,version,source,kind,license,tags,entry_point,content,tools,requires,provenance,labels,created_at,updated_at
		FROM skills WHERE id=?`, id)
	return scanSkill(row)
}

func (r *SQLiteRepository) ListSkills(ctx context.Context) ([]domain.Skill, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,description,version,source,kind,license,tags,entry_point,content,tools,requires,provenance,labels,created_at,updated_at
		FROM skills ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Skill
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) DeleteSkill(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM skill_bindings WHERE skill_id=?`, id)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM skills WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSkill(scanner interface{ Scan(...any) error }) (domain.Skill, error) {
	var s domain.Skill
	var tags, tools, requires, provenance, labels, created, updated string
	err := scanner.Scan(&s.ID, &s.Name, &s.Description, &s.Version, &s.Source, &s.Kind, &s.License,
		&tags, &s.EntryPoint, &s.Content, &tools, &requires, &provenance, &labels, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Skill{}, ErrNotFound
	}
	if err != nil {
		return domain.Skill{}, err
	}
	_ = json.Unmarshal([]byte(tags), &s.Tags)
	_ = json.Unmarshal([]byte(tools), &s.Tools)
	_ = json.Unmarshal([]byte(requires), &s.Requires)
	_ = json.Unmarshal([]byte(provenance), &s.Provenance)
	_ = json.Unmarshal([]byte(labels), &s.Labels)
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return s, nil
}

func (r *SQLiteRepository) BindSkill(ctx context.Context, b domain.SkillBinding) (domain.SkillBinding, error) {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO skill_bindings (id, profile_id, skill_id, enabled, created_at)
		VALUES (?,?,?,?,?)`, b.ID, b.ProfileID, b.SkillID, boolInt(b.Enabled), b.CreatedAt.Format(time.RFC3339Nano))
	return b, err
}

func (r *SQLiteRepository) ListSkillBindings(ctx context.Context, profileID string) ([]domain.SkillBinding, error) {
	var rows *sql.Rows
	var err error
	if profileID == "" {
		rows, err = r.db.QueryContext(ctx, `SELECT id, profile_id, skill_id, enabled, created_at FROM skill_bindings ORDER BY created_at DESC`)
	} else {
		rows, err = r.db.QueryContext(ctx, `SELECT id, profile_id, skill_id, enabled, created_at FROM skill_bindings WHERE profile_id=? ORDER BY created_at DESC`, profileID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SkillBinding
	for rows.Next() {
		var b domain.SkillBinding
		var enabled int
		var created string
		if err := rows.Scan(&b.ID, &b.ProfileID, &b.SkillID, &enabled, &created); err != nil {
			return nil, err
		}
		b.Enabled = enabled == 1
		b.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) UnbindSkill(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM skill_bindings WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
