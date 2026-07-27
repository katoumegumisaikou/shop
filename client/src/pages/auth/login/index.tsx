import { useState } from 'react'
import { Button, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'

import './index.scss'

export default function LoginPage() {
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState<boolean>(false)

  const handleLogin = () => {
    if (!/^1\d{10}$/.test(phone)) {
      Taro.showToast({
        title: '请输入正确的手机号',
        icon: 'none',
      })
      return
    }

    if (password.length < 6) {
      Taro.showToast({
        title: '密码不能少于 6 位',
        icon: 'none',
      })
      return
    }

    // 设置登录状态，避免重复点击
    setLoading(true)

    // todo: 这里去请求后端接口
    Taro.showToast({title:'登录成功',icon:'success',duration:1000})

    setLoading(false)
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
          <View className='login-card__link'>还没有账号？请先注册</View>
          <View className='login-card__link login-card__link--forgot'>忘记密码</View>
        </View>
      </View>
    </View>
  )
}
