export default defineAppConfig({
  pages: [
    'pages/auth/login/index',
    'pages/auth/register/index',
    'pages/auth/reset-password/index',
    'pages/index/index'
  ],
  window: {
    backgroundTextStyle: 'light',
    navigationBarBackgroundColor: '#fff',
    navigationBarTitleText: 'WeChat',
    navigationBarTextStyle: 'black'
  }
})
