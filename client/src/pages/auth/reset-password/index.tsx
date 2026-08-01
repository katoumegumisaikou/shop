import { useState } from 'react'
import { Button, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'

import '../login/index.scss'
import { ResetPassword, SendSmsCode } from '@/services/auth'

export default function ResetPasswordPage() {
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSendCode = async () => {
    if (!/^1\d{10}$/.test(phone)) {
      Taro.showToast({ title: '请输入正确的手机号', icon: 'none' })
      return
    }
    try {
      const res = await SendSmsCode(phone, 'reset')
      setCode(res.code)
      Taro.showToast({ title: '验证码已发送', icon: 'success' })
    } catch {
      Taro.showToast({ title: '发送失败，请重试', icon: 'none' })
    }
  }

  const handleReset = async () => {
    if (!/^1\d{10}$/.test(phone)) {
      Taro.showToast({ title: '请输入正确的手机号', icon: 'none' })
      return
    }
    if (password.length < 6) {
      Taro.showToast({ title: '密码不能少于 6 位', icon: 'none' })
      return
    }
    if (!code) {
      Taro.showToast({ title: '请输入验证码', icon: 'none' })
      return
    }

    setLoading(true)
    try {
      await ResetPassword(phone, code, password)
      Taro.showToast({ title: '密码重置成功', icon: 'success', duration: 1000 })
      setTimeout(() => Taro.navigateBack(), 1000)
    } catch (err: any) {
      Taro.showToast({ title: err.message || '重置失败', icon: 'none' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <View className='login-page'>
      <View className='login-card'>
        <View className='login-card__header'>
          <Text className='login-card__title'>重置密码</Text>
          <Text className='login-card__description'>设置新的登录密码</Text>
        </View>

        <View className='login-card__form'>
          <View className='login-card__field'>
            <Text className='login-card__label'>手机号</Text>
            <Input
              className='login-card__input'
              type='number'
              maxlength={11}
              placeholder='请输入绑定的手机号'
              value={phone}
              onInput={(event) => setPhone(event.detail.value)}
            />
          </View>

          <View className='login-card__field'>
            <Text className='login-card__label'>验证码</Text>
            <Input
              className='login-card__input'
              type='number'
              maxlength={6}
              placeholder='请输入验证码'
              value={code}
              onInput={(event) => setCode(event.detail.value)}
            />
            <Button
              className='login-card__send-code'
              onClick={handleSendCode}
              size='mini'
            >
              发送
            </Button>
          </View>

          <View className='login-card__field'>
            <Text className='login-card__label'>新密码</Text>
            <Input
              className='login-card__input'
              password
              placeholder='请设置 6 位以上密码'
              value={password}
              onInput={(event) => setPassword(event.detail.value)}
            />
          </View>

          <Button
            className='login-card__submit'
            onClick={handleReset}
            loading={loading}
            disabled={loading}
          >
            重置密码
          </Button>
        </View>
      </View>
    </View>
  )
}
