import { describe, expect, it } from 'vitest'

import {
  CHANNEL_CONNECTION_INFO_LABEL,
  CHANNEL_CONNECTION_INFO_TYPE,
  encodeChannelConnectionInfo,
  parseChannelConnectionInfo,
} from './channel-connection-info'

describe('channel connection info', () => {
  it('includes the branded type and normalized URL when copied', () => {
    const encoded = encodeChannelConnectionInfo(
      'sk-example',
      'https://gate.vectorepoch.com///'
    )

    expect(JSON.parse(encoded)).toEqual({
      type: CHANNEL_CONNECTION_INFO_LABEL,
      _type: CHANNEL_CONNECTION_INFO_TYPE,
      key: 'sk-example',
      url: 'https://gate.vectorepoch.com',
    })
  })

  it('continues to parse previously copied connection info', () => {
    expect(
      parseChannelConnectionInfo(
        '{"_type":"newapi_channel_conn","key":"sk-old","url":"https://gate.vectorepoch.com"}'
      )
    ).toEqual({ key: 'sk-old', url: 'https://gate.vectorepoch.com' })
  })
})
