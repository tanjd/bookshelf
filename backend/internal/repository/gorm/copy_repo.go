package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/bookshelf/internal/models"
	"github.com/tanjd/bookshelf/internal/repository"
)

// CopyRepository is the GORM implementation of repository.CopyRepository.
type CopyRepository struct {
	db *gorm.DB
}

// NewCopyRepository creates a new CopyRepository.
func NewCopyRepository(db *gorm.DB) *CopyRepository {
	return &CopyRepository{db: db}
}

func (r *CopyRepository) Create(copy *models.Copy) error {
	return r.db.Create(copy).Error
}

func (r *CopyRepository) GetByID(id uint) (*models.Copy, error) {
	var copy models.Copy
	if err := r.db.First(&copy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &copy, nil
}

func (r *CopyRepository) GetByIDWithAssociations(id uint) (*models.Copy, error) {
	var copy models.Copy
	if err := r.db.Preload("Book").Preload("Owner").First(&copy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &copy, nil
}

func (r *CopyRepository) GetByIDWithOwner(id uint) (*models.Copy, error) {
	var copy models.Copy
	if err := r.db.Preload("Owner").First(&copy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &copy, nil
}

func (r *CopyRepository) ListByOwnerID(ownerID uint) ([]models.Copy, error) {
	var copies []models.Copy
	if err := r.db.Preload("Book").Where("owner_id = ?", ownerID).Find(&copies).Error; err != nil {
		return nil, err
	}
	return copies, nil
}

func (r *CopyRepository) CountByOwnerID(ownerID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Copy{}).Where("owner_id = ?", ownerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *CopyRepository) Save(copy *models.Copy) error {
	return r.db.Save(copy).Error
}

func (r *CopyRepository) Delete(copy *models.Copy) error {
	return r.db.Delete(copy).Error
}

func (r *CopyRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&models.Copy{}).Where("id = ?", id).Update("status", status).Error
}
