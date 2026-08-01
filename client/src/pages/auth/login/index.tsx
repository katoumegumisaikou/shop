import { useState } from 'react'
import { Button, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'

import './index.scss'
import { MpLogin, PhoneLogin } from '@/services/auth'
import { setToken, setRefreshToken } from '@/services/api'
import { isH5 } from '@/utils/env'

export default function LoginPage() {
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async () => {
    if (!/^1\d{10}$/.test(phone)) {
      Taro.showToast({ title: '请输入正确的手机号', icon: 'none' })
      return
    }

    if (password.length < 6) {
      Taro.showToast({ title: '密码不能少于 6 位', icon: 'none' })
      return
    }

    setLoading(true)

    try {
      if (isH5()) {
        await PhoneLogin(phone, password)
      } else {
        // 小程序：微信授权登录，手动存储 token
        const loginRes = await Taro.login()
        const res = await MpLogin(loginRes.code)
        setToken(res.access_token)
        setRefreshToken(res.refresh_token)
      }

      // 浏览器原生跳转，不依赖 Taro 路由
      window.location.href = window.location.origin + '/#/pages/index/index'
    } catch (err: any) {
      Taro.showToast({ title: err.message || '登录失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <View className='login-page'>
      <View className='login-card'>
        <View className='login-card__header'>
          <Text className='login-card__title'>欢迎登录</Text>
          <Text className='login-card__description'>登录后继续访问商城</Text>
        </View>

        <View className='login-card__form'>
          <View className='login-card__field'>
            <Text className='login-card__label'>手机号</Text>
            <Input
              className='login-card__input'
              type='number'
              maxlength={11}
              placeholder='请输入 11 位手机号'
              value={phone}
              onInput={(event) => setPhone(event.detail.value)}
            />
          </View>

          <View className='login-card__field'>
            <Text className='login-card__label'>密码</Text>
            <Input
              className='login-card__input'
              password
              placeholder='请输入密码'
              value={password}
              onInput={(event) => setPassword(event.detail.value)}
            />
          </View>

          <Button className='login-card__submit'
            onClick={handleLogin}
            loading={loading}
            disabled={loading}
          >
            登录
          </Button>
        </View>

        <View className='login-card__actions'>
          <View
            className='login-card__link'
            onClick={() => Taro.navigateTo({ url: '/pages/auth/register/index' })}
          >还没有账号？请先注册</View>
          <View
            className='login-card__link login-card__link--forgot'
            onClick={() => Taro.navigateTo({ url: '/pages/auth/reset-password/index' })}
          >忘记密码</View>
        </View>
      </View>
    </View>
  )
}
