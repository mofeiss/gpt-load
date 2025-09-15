import type { Category } from "@/types/models";
import http from "@/utils/http";

export const categoriesApi = {
  // 获取所有分类
  async getCategories(): Promise<Category[]> {
    const res = await http.get("/categories");
    return res.data || [];
  },

  // 创建分类
  async createCategory(category: Partial<Category>): Promise<Category> {
    const res = await http.post("/categories", category);
    return res.data;
  },

  // 更新分类名称
  async updateCategory(categoryId: number, category: Partial<Category>): Promise<Category> {
    const res = await http.put(`/categories/${categoryId}`, category);
    return res.data;
  },

  // 删除分类（将分类下的所有group移到归档）
  deleteCategory(categoryId: number): Promise<void> {
    return http.delete(`/categories/${categoryId}`);
  },

  // 批量更新分类排序
  async updateCategoriesOrder(categories: { id: number; sort: number }[]): Promise<void> {
    return http.put("/categories/order", { categories });
  },
};