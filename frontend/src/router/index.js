import { createRouter, createWebHashHistory } from 'vue-router'
import LoginView from '../views/LoginView.vue'
import AppLayout from '../components/AppLayout.vue'
import RecordsView from '../views/RecordsView.vue'
import SummaryView from '../views/SummaryView.vue'
import ReportView from '../views/ReportView.vue'
import AdminUsersView from '../views/AdminUsersView.vue'
import LogsView from '../views/LogsView.vue'
import { isLoggedIn, isAdmin } from '../api/auth'

const routes = [
  { path: '/login', name: 'login', component: LoginView },
  {
    path: '/',
    component: AppLayout,
    children: [
      { path: '', redirect: '/records' },
      { path: 'records', name: 'records', component: RecordsView, meta: { title: '记账' } },
      { path: 'summary', name: 'summary', component: SummaryView, meta: { title: '汇总' } },
      { path: 'report', name: 'report', component: ReportView, meta: { title: '报表' } },
      { path: 'users', name: 'users', component: AdminUsersView, meta: { title: '用户管理', admin: true } },
      { path: 'logs', name: 'logs', component: LogsView, meta: { title: '操作日志', admin: true } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.name !== 'login' && !isLoggedIn()) {
    return { name: 'login' }
  }
  if (to.name === 'login' && isLoggedIn()) {
    return { path: '/' }
  }
  if (to.meta && to.meta.admin && !isAdmin()) {
    return { path: '/' }
  }
  return true
})

export default router
