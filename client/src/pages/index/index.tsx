import { useState } from 'react'
import { View, Text, Button } from '@tarojs/components'
import Taro, { useDidShow } from '@tarojs/taro'

import './index.scss'
import { GetMe, type User } from '@/services/auth'
import { isH5 } from '@/utils/env'

export default function Index() {
  const [user, setUser] = useState<User | null>(null)

  const fetchUser = async () => {
    try {
      const res = await GetMe()
      setUser(res)
    } catch {
      setUser(null)
    }
  }

  // 每次页面显示时刷新用户状态（登录后 switchTab 回来时触发）
  useDidShow(() => {
    fetchUser()
  })

  const handleLogin = () => {
    Taro.navigateTo({ url: '/pages/auth/login/index' })
  }

  const handleLogout = () => {
    if (isH5()) {
      Taro.request({
        url: 'http://localhost:8080/api/v1/c/auth/logout',
        method: 'POST',
      }).finally(() => {
        Taro.redirectTo({ url: '/pages/index/index' })
      })
    } else {
      Taro.redirectTo({ url: '/pages/auth/login/index' })
    }
  }

  return (
    <View className='home'>
      {/* 顶部用户区域 */}
      <View className='home__header'>
        {user ? (
          <View className='home__user'>
            <View className='home__avatar'>
              <Text className='home__avatar-text'>
                {(user.nickname || 'U')[0]}
              </Text>
            </View>
            <View className='home__user-info'>
              <Text className='home__greeting'>
                你好，{user.nickname || '用户'}
              </Text>
              <Text className='home__phone'>{user.phone}</Text>
            </View>
            <Button className='home__logout-btn' onClick={handleLogout} size='mini'>
              退出
            </Button>
          </View>
        ) : (
          <View className='home__guest' onClick={handleLogin}>
            <View className='home__avatar home__avatar--placeholder' />
            <Text className='home__login-hint'>点击登录</Text>
          </View>
        )}
      </View>

      {/* 快捷入口 */}
      <View className='home__grid'>
        <View className='home__card'>
          <Text className='home__card-icon'>🛍️</Text>
          <Text className='home__card-text'>全部商品</Text>
        </View>
        <View className='home__card'>
          <Text className='home__card-icon'>🛒</Text>
          <Text className='home__card-text'>购物车</Text>
        </View>
        <View className='home__card'>
          <Text className='home__card-icon'>📦</Text>
          <Text className='home__card-text'>我的订单</Text>
        </View>
        <View className='home__card'>
          <Text className='home__card-icon'>👤</Text>
          <Text className='home__card-text'>个人中心</Text>
        </View>
      </View>

      {/* 余额 */}
      {user && (
        <View className='home__section'>
          <Text className='home__section-title'>我的余额</Text>
          <View className='home__balance'>
            <Text className='home__balance-amount'>
              ¥{(user.balance_cents / 100).toFixed(2)}
            </Text>
          </View>
        </View>
      )}

      {/* 占位 */}
      <View className='home__placeholder'>
        <Text className='home__placeholder-text'>更多功能敬请期待</Text>
      </View>
    </View>
  )
}
