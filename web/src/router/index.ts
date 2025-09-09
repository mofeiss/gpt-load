import { useAuthService } from "@/services/auth";
import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import Layout from "@/components/Layout.vue";

const routes: Array<RouteRecordRaw> = [
  {
    path: "/",
    component: Layout,
    children: [
      {
        path: "",
        name: "dashboard",
        component: () => import("@/views/Dashboard.vue"),
      },
      {
        path: "keys",
        name: "keys",
        component: () => import("@/views/Keys.vue"),
      },
      {
        path: "logs",
        name: "logs",
        component: () => import("@/views/Logs.vue"),
      },
      {
        path: "settings",
        name: "settings",
        component: () => import("@/views/Settings.vue"),
      },
    ],
  },
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/Login.vue"),
  },
];

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
});

const { checkLogin } = useAuthService();

router.beforeEach((to, _from, next) => {
  const loggedIn = checkLogin();
  if (to.path !== "/login" && !loggedIn) {
    return next({ path: "/login" });
  }

  if (to.path === "/login" && loggedIn) {
    return next({ path: "/" });
  }

  next();
});

// 应用启动时检查 localStorage 中的保存视图并跳转
const savedView = localStorage.getItem("lastActiveView");
if (
  savedView &&
  savedView !== router.currentRoute.value.name &&
  router.currentRoute.value.path === "/"
) {
  router.push({ name: savedView });
}

export default router;
