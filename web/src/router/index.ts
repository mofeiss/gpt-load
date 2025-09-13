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
        path: "ccr",
        name: "ccr",
        component: () => import("@/views/CCR.vue"),
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

// 延迟恢复导航状态，确保路由已经完全初始化
setTimeout(() => {
  const savedView = localStorage.getItem("lastActiveView");
  const currentRoute = router.currentRoute.value;

  // 检查是否需要恢复视图
  if (
    savedView &&
    savedView !== currentRoute.name &&
    // 在主页路径才需要恢复，避免干扰其他页面访问
    (currentRoute.path === "/" ||
      currentRoute.path === "/index.html" ||
      currentRoute.name === "dashboard")
  ) {
    router.push({ name: savedView });
  }
}, 0);

export default router;
