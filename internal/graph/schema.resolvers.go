package graph

import (
	"context"
	"strconv"
	"time"

	"genealogy_go/internal/model"
)

// Members 获取所有家族成员
func (r *queryResolver) Members(ctx context.Context) ([]*model.Member, error) {
	var members []*model.Member
	if err := r.db.Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

// Member 根据ID获取家族成员
func (r *queryResolver) Member(ctx context.Context, id string) (*model.Member, error) {
	var member model.Member
	if err := r.db.First(&member, id).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

// Photos 获取指定成员的照片
func (r *queryResolver) Photos(ctx context.Context, memberID string) ([]*model.Photo, error) {
	var photos []*model.Photo
	if err := r.db.Where("member_id = ?", memberID).Find(&photos).Error; err != nil {
		return nil, err
	}
	return photos, nil
}

// Events 获取指定成员的事件
func (r *queryResolver) Events(ctx context.Context, memberID string) ([]*model.Event, error) {
	var events []*model.Event
	if err := r.db.Where("member_id = ?", memberID).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// CreateMember 创建新的家族成员
func (r *mutationResolver) CreateMember(ctx context.Context, input model.MemberInput) (*model.Member, error) {
	birthDate, err := time.Parse("2006-01-02", input.BirthDate)
	if err != nil {
		return nil, err
	}

	var deathDate *time.Time
	if input.DeathDate != nil {
		parsedDate, err := time.Parse("2006-01-02", *input.DeathDate)
		if err != nil {
			return nil, err
		}
		deathDate = &parsedDate
	}

	member := &model.Member{
		Name:         input.Name,
		Gender:       input.Gender,
		BirthDate:    birthDate,
		DeathDate:    deathDate,
		BirthPlace:   input.BirthPlace,
		CurrentPlace: input.CurrentPlace,
		Education:    input.Education,
		Occupation:   input.Occupation,
		Description:  input.Description,
	}

	if input.FatherID != nil {
		fatherID, _ := strconv.ParseUint(*input.FatherID, 10, 32)
		member.FatherID = (*uint)(&fatherID)
	}

	if input.MotherID != nil {
		motherID, _ := strconv.ParseUint(*input.MotherID, 10, 32)
		member.MotherID = (*uint)(&motherID)
	}

	if input.SpouseID != nil {
		spouseID, _ := strconv.ParseUint(*input.SpouseID, 10, 32)
		member.SpouseID = (*uint)(&spouseID)
	}

	if err := r.db.Create(member).Error; err != nil {
		return nil, err
	}

	return member, nil
}

// UpdateMember 更新家族成员信息
func (r *mutationResolver) UpdateMember(ctx context.Context, id string, input model.MemberInput) (*model.Member, error) {
	var member model.Member
	if err := r.db.First(&member, id).Error; err != nil {
		return nil, err
	}

	birthDate, err := time.Parse("2006-01-02", input.BirthDate)
	if err != nil {
		return nil, err
	}

	var deathDate *time.Time
	if input.DeathDate != nil {
		parsedDate, err := time.Parse("2006-01-02", *input.DeathDate)
		if err != nil {
			return nil, err
		}
		deathDate = &parsedDate
	}

	updates := map[string]interface{}{
		"name":          input.Name,
		"gender":        input.Gender,
		"birth_date":    birthDate,
		"death_date":    deathDate,
		"birth_place":   input.BirthPlace,
		"current_place": input.CurrentPlace,
		"education":     input.Education,
		"occupation":    input.Occupation,
		"description":   input.Description,
	}

	if input.FatherID != nil {
		fatherID, _ := strconv.ParseUint(*input.FatherID, 10, 32)
		updates["father_id"] = (*uint)(&fatherID)
	}

	if input.MotherID != nil {
		motherID, _ := strconv.ParseUint(*input.MotherID, 10, 32)
		updates["mother_id"] = (*uint)(&motherID)
	}

	if input.SpouseID != nil {
		spouseID, _ := strconv.ParseUint(*input.SpouseID, 10, 32)
		updates["spouse_id"] = (*uint)(&spouseID)
	}

	if err := r.db.Model(&member).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &member, nil
}

// DeleteMember 删除家族成员
func (r *mutationResolver) DeleteMember(ctx context.Context, id string) (bool, error) {
	if err := r.db.Delete(&model.Member{}, id).Error; err != nil {
		return false, err
	}
	return true, nil
}

// CreatePhoto 创建新的照片记录
func (r *mutationResolver) CreatePhoto(ctx context.Context, input model.PhotoInput) (*model.Photo, error) {
	takeDate, err := time.Parse("2006-01-02", input.TakeDate)
	if err != nil {
		return nil, err
	}

	photo := &model.Photo{
		MemberID:    input.MemberID,
		URL:         input.URL,
		Description: input.Description,
		TakeDate:    takeDate,
	}

	if err := r.db.Create(photo).Error; err != nil {
		return nil, err
	}

	return photo, nil
}

// UpdatePhoto 更新照片信息
func (r *mutationResolver) UpdatePhoto(ctx context.Context, id string, input model.PhotoInput) (*model.Photo, error) {
	var photo model.Photo
	if err := r.db.First(&photo, id).Error; err != nil {
		return nil, err
	}

	takeDate, err := time.Parse("2006-01-02", input.TakeDate)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"member_id":  input.MemberID,
		"url":        input.URL,
		"description": input.Description,
		"take_date":  takeDate,
	}

	if err := r.db.Model(&photo).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &photo, nil
}

// DeletePhoto 删除照片
func (r *mutationResolver) DeletePhoto(ctx context.Context, id string) (bool, error) {
	if err := r.db.Delete(&model.Photo{}, id).Error; err != nil {
		return false, err
	}
	return true, nil
}

// CreateEvent 创建新的事件记录
func (r *mutationResolver) CreateEvent(ctx context.Context, input model.EventInput) (*model.Event, error) {
	eventDate, err := time.Parse("2006-01-02", input.EventDate)
	if err != nil {
		return nil, err
	}

	event := &model.Event{
		MemberID:    input.MemberID,
		Title:       input.Title,
		Description: input.Description,
		EventDate:   eventDate,
		EventType:   input.EventType,
		Location:    input.Location,
	}

	if err := r.db.Create(event).Error; err != nil {
		return nil, err
	}

	return event, nil
}

// UpdateEvent 更新事件信息
func (r *mutationResolver) UpdateEvent(ctx context.Context, id string, input model.EventInput) (*model.Event, error) {
	var event model.Event
	if err := r.db.First(&event, id).Error; err != nil {
		return nil, err
	}

	eventDate, err := time.Parse("2006-01-02", input.EventDate)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"member_id":  input.MemberID,
		"title":      input.Title,
		"description": input.Description,
		"event_date": eventDate,
		"event_type": input.EventType,
		"location":   input.Location,
	}

	if err := r.db.Model(&event).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &event, nil
}

// DeleteEvent 删除事件
func (r *mutationResolver) DeleteEvent(ctx context.Context, id string) (bool, error) {
	if err := r.db.Delete(&model.Event{}, id).Error; err != nil {
		return false, err
	}
	return true, nil
} 