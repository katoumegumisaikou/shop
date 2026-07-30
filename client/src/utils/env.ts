import Taro from '@tarojs/taro'

/** 当前是否运行在微信小程序环境 */
export function isWeapp(): boolean {
  return Taro.getEnv() === 'WEAPP'
}

/** 当前是否运行在 H5 / 浏览器环境 */
export function isH5(): boolean {
  return Taro.getEnv() === 'WEB'
}
