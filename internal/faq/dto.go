package faq

type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type UpdateCategoryRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Order       *int    `json:"order"`
}

type CreateItemRequest struct {
	CategoryID uint   `json:"category_id" binding:"required"`
	Question   string `json:"question" binding:"required"`
	Answer     string `json:"answer" binding:"required"`
}

type UpdateItemRequest struct {
	Question *string `json:"question"`
	Answer   *string `json:"answer"`
	Order    *int    `json:"order"`
}

type ReorderItemRequest struct {
	ItemID uint `json:"item_id" binding:"required"`
	Order  int  `json:"order" binding:"required"`
}
