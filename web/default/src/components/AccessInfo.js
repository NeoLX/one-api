import React, { useEffect, useState } from 'react';
import { Button, Form, Header, Loader, Message, Segment } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { API, copy, showError, showSuccess } from '../helpers';

const ENDPOINTS = [
  '/v1/chat/completions',
  '/v1/completions',
  '/v1/embeddings',
  '/v1/audio/speech',
  '/v1/audio/transcriptions',
  '/v1/images/generations',
];

const AccessInfo = () => {
  const { t } = useTranslation();
  const [models, setModels] = useState([]);
  const [loading, setLoading] = useState(true);

  const baseURL = `${window.location.origin}/v1`;

  const loadModels = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/user/available_models');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setModels(data || []);
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadModels().then();
  }, []);

  const copyText = (text) => {
    copy(text).then((ok) => {
      if (ok) {
        showSuccess(t('token.messages.copy_success'));
      } else {
        showError(t('token.access_info.copy_failed'));
      }
    });
  };

  const curlExample = `curl ${baseURL}/chat/completions \\
  -H "Authorization: Bearer <你的令牌>" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"${models[0] || '模型 id'}","messages":[{"role":"user","content":"你好"}]}'`;

  return (
    <Segment>
      <Header as='h3'>{t('token.access_info.title')}</Header>
      <Message info>{t('token.access_info.token_hint')}</Message>
      <Form>
        <Form.Field>
          <label>{t('token.access_info.base_url')}</label>
          <Form.Input
            readOnly
            value={baseURL}
            action={{
              type: 'button',
              content: t('token.access_info.copy'),
              onClick: () => copyText(baseURL),
            }}
          />
        </Form.Field>
        <Form.Field>
          <label>{t('token.access_info.models')}</label>
          {loading ? (
            <Loader active inline size='small' />
          ) : models.length === 0 ? (
            <Message size='small'>{t('token.access_info.no_models')}</Message>
          ) : (
            <>
              <Segment
                secondary
                style={{
                  maxHeight: '240px',
                  overflowY: 'auto',
                  padding: '0.5em 0.75em',
                }}
              >
                {models.map((model, idx) => (
                  <div
                    key={model}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      gap: '0.75em',
                      padding: '0.25em 0',
                      borderTop: idx === 0 ? 'none' : '1px solid rgba(34,36,38,.08)',
                    }}
                  >
                    <span
                      style={{
                        fontFamily: 'monospace',
                        wordBreak: 'break-all',
                      }}
                    >
                      {model}
                    </span>
                    <Button
                      type='button'
                      basic
                      size='mini'
                      icon='copy'
                      title={t('token.access_info.copy')}
                      onClick={() => copyText(model)}
                    />
                  </div>
                ))}
              </Segment>
              <Button
                type='button'
                basic
                size='small'
                onClick={() => copyText(models.join('\n'))}
              >
                {t('token.access_info.copy_all')}
              </Button>
            </>
          )}
        </Form.Field>
        <Form.Field>
          <label>{t('token.access_info.example')}</label>
          <Segment secondary style={{ overflowX: 'auto' }}>
            <pre style={{ margin: 0 }}>{curlExample}</pre>
          </Segment>
          <Button
            type='button'
            basic
            size='small'
            onClick={() => copyText(curlExample)}
          >
            {t('token.access_info.copy')}
          </Button>
        </Form.Field>
        <Form.Field>
          <label>{t('token.access_info.endpoints')}</label>
          <Segment secondary style={{ fontFamily: 'monospace' }}>
            {ENDPOINTS.map((endpoint) => (
              <div key={endpoint}>{endpoint}</div>
            ))}
          </Segment>
        </Form.Field>
      </Form>
    </Segment>
  );
};

export default AccessInfo;
