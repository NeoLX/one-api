import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Checkbox,
  Divider,
  Form,
  Loader,
  Message,
  Segment,
} from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showInfo, showSuccess } from '../../helpers';
import { CHANNEL_OPTIONS } from '../../constants/channel.constants';

const Proxy = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [options, setOptions] = useState([]);
  // selection: { [type]: string[] }，空数组表示该供应商全部模型
  const [selection, setSelection] = useState({});

  const typeLabels = useMemo(() => {
    const map = {};
    CHANNEL_OPTIONS.forEach((option) => {
      map[option.value] = option.text;
    });
    return map;
  }, []);

  const getProviderLabel = (option) =>
    typeLabels[option.type] || option.name || String(option.type);

  const loadProxy = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/user/proxy');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const loadedOptions = (data && data.options) || [];
      setOptions(loadedOptions);
      setEnabled(Boolean(data && data.restricted));
      const config = (data && data.config) || {};
      const nextSelection = {};
      Object.keys(config).forEach((typeKey) => {
        nextSelection[typeKey] = config[typeKey] || [];
      });
      setSelection(nextSelection);
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadProxy().then();
  }, []);

  const isProviderChecked = (type) => `${type}` in selection;

  const toggleProvider = (type, checked) => {
    setSelection((current) => {
      const next = { ...current };
      if (checked) {
        next[`${type}`] = [];
      } else {
        delete next[`${type}`];
      }
      return next;
    });
  };

  const isModelChecked = (type, model) => {
    const models = selection[`${type}`];
    if (!models || models.length === 0) {
      return false;
    }
    return models.includes(model);
  };

  const setProviderModels = (type, models) => {
    setSelection((current) => ({ ...current, [`${type}`]: models }));
  };

  const toggleModel = (type, model, checked, providerModels) => {
    const current = selection[`${type}`] || [];
    if (checked) {
      setProviderModels(type, providerModels.filter((m) => current.includes(m) || m === model));
      return;
    }
    setProviderModels(type, current.filter((m) => m !== model));
  };

  const selectAllModels = (type, providerModels) => {
    setProviderModels(type, [...providerModels]);
  };

  const clearModels = (type) => {
    // 留空数组 = 该供应商全部模型
    setProviderModels(type, []);
  };

  const submit = async () => {
    if (enabled && Object.keys(selection).length === 0) {
      showInfo(t('proxy.none_selected'));
      return;
    }
    setSaving(true);
    try {
      const config = enabled ? selection : {};
      const res = await API.put('/api/user/proxy', { config });
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('proxy.saved'));
      await loadProxy();
    } catch (error) {
      showError(error.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header'>{t('proxy.title')}</Card.Header>
          <Message info>{t('proxy.hint')}</Message>
          {loading ? (
            <Loader active inline='centered' />
          ) : (
            <Form>
              <Form.Field>
                <Checkbox
                  toggle
                  label={t('proxy.enable')}
                  checked={enabled}
                  onChange={(e, { checked }) => setEnabled(Boolean(checked))}
                />
              </Form.Field>
              {enabled && options.length === 0 && (
                <Message warning>{t('proxy.no_options')}</Message>
              )}
              {enabled &&
                options.map((option) => (
                  <Segment key={option.type} secondary>
                    <Checkbox
                      label={getProviderLabel(option)}
                      checked={isProviderChecked(option.type)}
                      onChange={(e, { checked }) =>
                        toggleProvider(option.type, Boolean(checked))
                      }
                    />
                    {isProviderChecked(option.type) && (
                      <>
                        <Button
                          type='button'
                          basic
                          size='mini'
                          style={{ marginLeft: '12px' }}
                          onClick={() => selectAllModels(option.type, option.models)}
                        >
                          {t('proxy.select_all')}
                        </Button>
                        <Button
                          type='button'
                          basic
                          size='mini'
                          onClick={() => clearModels(option.type)}
                        >
                          {t('proxy.all_models')}
                        </Button>
                        <Divider />
                        <Form.Group inline style={{ flexWrap: 'wrap' }}>
                          {option.models.map((model) => (
                            <Form.Field key={model} style={{ minWidth: '200px' }}>
                              <Checkbox
                                label={model}
                                checked={isModelChecked(option.type, model)}
                                onChange={(e, { checked }) =>
                                  toggleModel(
                                    option.type,
                                    model,
                                    Boolean(checked),
                                    option.models
                                  )
                                }
                              />
                            </Form.Field>
                          ))}
                        </Form.Group>
                        {(selection[`${option.type}`] || []).length === 0 && (
                          <Message size='small'>{t('proxy.all_models_tip')}</Message>
                        )}
                      </>
                    )}
                  </Segment>
                ))}
              <Button type='button' positive loading={saving} onClick={submit}>
                {t('proxy.save')}
              </Button>
            </Form>
          )}
        </Card.Content>
      </Card>
    </div>
  );
};

export default Proxy;
