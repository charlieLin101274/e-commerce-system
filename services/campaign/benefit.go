package campaign

import (
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
)

func CalculateBenefit(benefitType models.BenefitType, value int64, maximum *int64, amount int64) (models.BenefitResult, error) {
	if amount < 0 || value <= 0 {
		return models.BenefitResult{}, apperror.ErrInvalidInput
	}
	var discount int64
	switch benefitType {
	case models.BenefitTypeFixedAmount:
		if maximum != nil {
			return models.BenefitResult{}, apperror.ErrInvalidInput
		}
		discount = value
	case models.BenefitTypePercentage:
		if value > 100 || (maximum != nil && *maximum <= 0) {
			return models.BenefitResult{}, apperror.ErrInvalidInput
		}
		// Split the calculation to avoid overflowing int64 for large amounts.
		discount = (amount/100)*value + (amount%100)*value/100
		if maximum != nil && discount > *maximum {
			discount = *maximum
		}
	default:
		return models.BenefitResult{}, apperror.ErrInvalidInput
	}
	if discount > amount {
		discount = amount
	}
	return models.BenefitResult{OriginalAmount: amount, DiscountAmount: discount, FinalAmount: amount - discount}, nil
}
