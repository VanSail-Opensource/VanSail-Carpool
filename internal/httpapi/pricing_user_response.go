package httpapi

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/model"

type userProductPriceResponse struct {
	model.UserProductPrice
	Coupons []couponGrantResponse `json:"coupons"`
}
type userPricePageResponse struct {
	Items      []userProductPriceResponse `json:"items"`
	NextCursor string                     `json:"nextCursor"`
}

func mapUserPricePage(page model.UserPricePage) userPricePageResponse {
	out := userPricePageResponse{Items: make([]userProductPriceResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, item := range page.Items {
		grants := make([]couponGrantResponse, 0, len(item.Coupons))
		for _, grant := range item.Coupons {
			grants = append(grants, mapCouponGrant(grant))
		}
		out.Items = append(out.Items, userProductPriceResponse{UserProductPrice: item, Coupons: grants})
	}
	return out
}
