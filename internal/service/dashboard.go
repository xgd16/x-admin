package service

import (
	"context"
	"time"

	"x-admin/internal/model"
)

type DashboardSummary struct {
	Users      int64                `json:"users"`
	Active     int64                `json:"active_users"`
	Roles      int64                `json:"roles"`
	Menus      int64                `json:"menus"`
	Files      int64                `json:"files"`
	FileSize   int64                `json:"file_size"`
	Jobs       int64                `json:"jobs"`
	LoginTrend []model.TrendPoint   `json:"login_trend"`
	Recent     []model.OperationLog `json:"recent_operations"`
}

type DashboardService struct {
	users UserRepository
	roles RoleRepository
	menus MenuRepository
	files FileRepository
	jobs  JobRepository
	logs  LogRepository
	now   func() time.Time
}

func NewDashboardService(users UserRepository, roles RoleRepository, menus MenuRepository, files FileRepository, jobs JobRepository, logs LogRepository) *DashboardService {
	return &DashboardService{users: users, roles: roles, menus: menus, files: files, jobs: jobs, logs: logs, now: time.Now}
}

func (s *DashboardService) Summary(ctx context.Context) (*DashboardSummary, error) {
	out := &DashboardSummary{}
	var err error
	if out.Users, err = s.users.CountByStatus(ctx, nil); err != nil {
		return nil, err
	}
	enabled := model.StatusEnabled
	if out.Active, err = s.users.CountByStatus(ctx, &enabled); err != nil {
		return nil, err
	}
	if out.Roles, err = s.roles.CountAll(ctx); err != nil {
		return nil, err
	}
	if out.Menus, err = s.menus.CountAll(ctx); err != nil {
		return nil, err
	}
	if out.Files, err = s.files.CountAll(ctx); err != nil {
		return nil, err
	}
	if out.FileSize, err = s.files.TotalSize(ctx); err != nil {
		return nil, err
	}
	if out.Jobs, err = s.jobs.CountAll(ctx); err != nil {
		return nil, err
	}
	trend, err := s.logs.LoginTrend(ctx, 7)
	if err != nil {
		return nil, err
	}
	out.LoginTrend = fillTrend(trend, 7, s.now())
	recent, err := s.logs.RecentOperations(ctx, 6)
	if err != nil {
		return nil, err
	}
	if recent == nil {
		recent = []model.OperationLog{}
	}
	out.Recent = recent
	return out, nil
}

func fillTrend(points []model.TrendPoint, days int, now time.Time) []model.TrendPoint {
	byDate := make(map[string]int64, len(points))
	for _, p := range points {
		date := p.Date
		if len(date) > 10 {
			date = date[:10]
		}
		byDate[date] = p.Count
	}
	out := make([]model.TrendPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, model.TrendPoint{Date: date, Count: byDate[date]})
	}
	return out
}
