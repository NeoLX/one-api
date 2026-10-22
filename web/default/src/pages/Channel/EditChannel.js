import React, {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Button, Card, Divider, Form, Input, Message, Modal} from 'semantic-ui-react';
import {useNavigate, useParams} from 'react-router-dom';
import {API, copy, getChannelModels, showError, showInfo, showSuccess, verifyJSON,} from '../../helpers';
import {CHANNEL_OPTIONS} from '../../constants';
import {renderChannelTip} from '../../helpers/render';

const MODEL_MAPPING_EXAMPLE = {
  'gpt-3.5-turbo-0301': 'gpt-3.5-turbo',
  'gpt-4-0314': 'gpt-4',
  'gpt-4-32k-0314': 'gpt-4-32k',
};

function type2secretPrompt(type, t) {
  switch (type) {
    case 15:
      return t('channel.edit.key_prompts.zhipu');
    case 18:
      return t('channel.edit.key_prompts.spark');
    case 22:
      return t('channel.edit.key_prompts.fastgpt');
    case 23:
      return t('channel.edit.key_prompts.tencent');
    default:
      return t('channel.edit.key_prompts.default');
  }
}

const EditChannel = () => {
  const { t } = useTranslation();
  const params = useParams();
  const navigate = useNavigate();
  const channelId = params.id;
  const isEdit = channelId !== undefined;
  const [loading, setLoading] = useState(isEdit);
  const handleCancel = () => {
    navigate('/channel');
  };

  const originInputs = {
    name: '',
    type: 1,
    key: '',
    base_url: '',
    other: '',
    model_mapping: '',
    system_prompt: '',
    models: [],
    groups: ['default'],
  };
  const [batch, setBatch] = useState(false);
  const [inputs, setInputs] = useState(originInputs);
  const [originModelOptions, setOriginModelOptions] = useState([]);
  const [modelOptions, setModelOptions] = useState([]);
  const [groupOptions, setGroupOptions] = useState([]);
  const [basicModels, setBasicModels] = useState([]);
  const [fullModels, setFullModels] = useState([]);
  const [customModel, setCustomModel] = useState('');
  const [headerRows, setHeaderRows] = useState([]);
  const [fetchModelsLoading, setFetchModelsLoading] = useState(false);
  const [fetchedModels, setFetchedModels] = useState(null);
  const [fetchedModelSelection, setFetchedModelSelection] = useState([]);
  const [fetchedModelKeyword, setFetchedModelKeyword] = useState('');
  const [config, setConfig] = useState({
    region: '',
    sk: '',
    ak: '',
    user_id: '',
    vertex_ai_project_id: '',
    vertex_ai_adc: '',
  });
  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
    if (name === 'type') {
      let localModels = getChannelModels(value);
      if (inputs.models.length === 0) {
        setInputs((inputs) => ({ ...inputs, models: localModels }));
      }
      setBasicModels(localModels);
    }
  };

  const handleConfigChange = (e, { name, value }) => {
    setConfig((inputs) => ({ ...inputs, [name]: value }));
  };

  const buildConfig = () => {
    const cfg = { ...config };
    const headers = {};
    headerRows.forEach((row) => {
      const key = row.key.trim();
      if (key !== '') {
        headers[key] = row.value;
      }
    });
    if (Object.keys(headers).length > 0) {
      cfg.headers = headers;
    } else {
      delete cfg.headers;
    }
    return JSON.stringify(cfg);
  };

  const addHeaderRow = () => {
    setHeaderRows((rows) => [...rows, { key: '', value: '' }]);
  };

  const updateHeaderRow = (index, field, value) => {
    setHeaderRows((rows) =>
      rows.map((row, i) => (i === index ? { ...row, [field]: value } : row))
    );
  };

  const removeHeaderRow = (index) => {
    setHeaderRows((rows) => rows.filter((_, i) => i !== index));
  };

  // Ollama 不使用密钥
  const keylessProvider = inputs.type === 30;

  const fetchUpstreamModels = async () => {
    if (inputs.key === '' && !keylessProvider && !isEdit) {
      showInfo(t('channel.edit.messages.fetch_models_key_required'));
      return;
    }
    setFetchModelsLoading(true);
    try {
      const res = await API.post('/api/channel/fetch_models', {
        id: isEdit ? parseInt(channelId) : 0,
        type: inputs.type,
        base_url: inputs.base_url,
        key: inputs.key,
        config: buildConfig(),
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const models = (data && data.models) || [];
      if (models.length === 0) {
        showInfo(t('channel.edit.messages.fetch_models_empty'));
        return;
      }
      setFetchedModels(models);
      setFetchedModelSelection(models.filter((m) => inputs.models.includes(m)));
      setFetchedModelKeyword('');
    } catch (error) {
      showError(error.message);
    } finally {
      setFetchModelsLoading(false);
    }
  };

  const confirmFetchedModels = () => {
    const merged = [...inputs.models];
    let added = 0;
    fetchedModelSelection.forEach((model) => {
      if (!merged.includes(model)) {
        merged.push(model);
        added += 1;
      }
    });
    handleInputChange(null, { name: 'models', value: merged });
    setFetchedModels(null);
    setFetchedModelSelection([]);
    if (added > 0) {
      showSuccess(t('channel.edit.messages.fetch_models_success', { count: added }));
    }
  };

  const toggleFetchedModel = (model) => {
    setFetchedModelSelection((selection) =>
      selection.includes(model)
        ? selection.filter((m) => m !== model)
        : [...selection, model]
    );
  };

  const invertVisibleFetchedModels = () => {
    const visible = (fetchedModels || []).filter((m) =>
      m.toLowerCase().includes(fetchedModelKeyword.trim().toLowerCase())
    );
    setFetchedModelSelection((selection) => {
      const toRemove = visible.filter((m) => selection.includes(m));
      const toAdd = visible.filter((m) => !selection.includes(m));
      return [...selection.filter((m) => !toRemove.includes(m)), ...toAdd];
    });
  };

  const loadChannel = async () => {
    let res = await API.get(`/api/channel/${channelId}`);
    const { success, message, data } = res.data;
    if (success) {
      if (data.models === '') {
        data.models = [];
      } else {
        data.models = data.models.split(',');
      }
      if (data.group === '') {
        data.groups = [];
      } else {
        data.groups = data.group.split(',');
      }
      if (data.model_mapping !== '') {
        data.model_mapping = JSON.stringify(
          JSON.parse(data.model_mapping),
          null,
          2
        );
      }
      setInputs(data);
      if (data.config !== '') {
        const cfg = JSON.parse(data.config);
        const cfgHeaders = cfg.headers || {};
        delete cfg.headers;
        setConfig(cfg);
        setHeaderRows(
          Object.keys(cfgHeaders).map((key) => ({ key, value: cfgHeaders[key] }))
        );
      }
      setBasicModels(getChannelModels(data.type));
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const fetchModels = async () => {
    try {
      let res = await API.get(`/api/channel/models`);
      let localModelOptions = res.data.data.map((model) => ({
        key: model.id,
        text: model.id,
        value: model.id,
      }));
      setOriginModelOptions(localModelOptions);
      setFullModels(res.data.data.map((model) => model.id));
    } catch (error) {
      showError(error.message);
    }
  };

  const fetchGroups = async () => {
    try {
      let res = await API.get(`/api/group/`);
      setGroupOptions(
        res.data.data.map((group) => ({
          key: group,
          text: group,
          value: group,
        }))
      );
    } catch (error) {
      showError(error.message);
    }
  };

  useEffect(() => {
    let localModelOptions = [...originModelOptions];
    inputs.models.forEach((model) => {
      if (!localModelOptions.find((option) => option.key === model)) {
        localModelOptions.push({
          key: model,
          text: model,
          value: model,
        });
      }
    });
    setModelOptions(localModelOptions);
  }, [originModelOptions, inputs.models]);

  useEffect(() => {
    if (isEdit) {
      loadChannel().then();
    } else {
      let localModels = getChannelModels(inputs.type);
      setBasicModels(localModels);
    }
    fetchModels().then();
    fetchGroups().then();
  }, []);

  const submit = async () => {
    if (inputs.key === '') {
      if (config.ak !== '' && config.sk !== '' && config.region !== '') {
        inputs.key = `${config.ak}|${config.sk}|${config.region}`;
      } else if (
        config.region !== '' &&
        config.vertex_ai_project_id !== '' &&
        config.vertex_ai_adc !== ''
      ) {
        inputs.key = `${config.region}|${config.vertex_ai_project_id}|${config.vertex_ai_adc}`;
      }
    }
    if (!isEdit && (inputs.name === '' || inputs.key === '')) {
      showInfo(t('channel.edit.messages.name_required'));
      return;
    }
    if (inputs.type !== 43 && inputs.models.length === 0) {
      showInfo(t('channel.edit.messages.models_required'));
      return;
    }
    if (inputs.model_mapping !== '' && !verifyJSON(inputs.model_mapping)) {
      showInfo(t('channel.edit.messages.model_mapping_invalid'));
      return;
    }
    let localInputs = { ...inputs };
    if (localInputs.key === 'undefined|undefined|undefined') {
      localInputs.key = ''; // prevent potential bug
    }
    if (localInputs.base_url && localInputs.base_url.endsWith('/')) {
      localInputs.base_url = localInputs.base_url.slice(
        0,
        localInputs.base_url.length - 1
      );
    }
    if (localInputs.type === 3 && localInputs.other === '') {
      localInputs.other = '2024-03-01-preview';
    }
    let res;
    localInputs.models = localInputs.models.join(',');
    localInputs.group = localInputs.groups.join(',');
    localInputs.config = buildConfig();
    if (isEdit) {
      res = await API.put(`/api/channel/`, {
        ...localInputs,
        id: parseInt(channelId),
      });
    } else {
      res = await API.post(`/api/channel/`, localInputs);
    }
    const { success, message } = res.data;
    if (success) {
      if (isEdit) {
        showSuccess(t('channel.edit.messages.update_success'));
      } else {
        showSuccess(t('channel.edit.messages.create_success'));
        setInputs(originInputs);
      }
    } else {
      showError(message);
    }
  };

  const addCustomModel = () => {
    if (customModel.trim() === '') return;
    if (inputs.models.includes(customModel)) return;
    let localModels = [...inputs.models];
    localModels.push(customModel);
    let localModelOptions = [];
    localModelOptions.push({
      key: customModel,
      text: customModel,
      value: customModel,
    });
    setModelOptions((modelOptions) => {
      return [...modelOptions, ...localModelOptions];
    });
    setCustomModel('');
    handleInputChange(null, { name: 'models', value: localModels });
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header'>
            {isEdit
              ? t('channel.edit.title_edit')
              : t('channel.edit.title_create')}
          </Card.Header>
          <Form loading={loading} autoComplete='new-password'>
            <Divider horizontal>{t('channel.edit.section_provider')}</Divider>
            <Form.Field>
              <Form.Select
                label={t('channel.edit.type')}
                name='type'
                required
                search
                options={CHANNEL_OPTIONS}
                value={inputs.type}
                onChange={handleInputChange}
              />
            </Form.Field>
            <Form.Field>
              <Form.Input
                label={t('channel.edit.name')}
                name='name'
                placeholder={t('channel.edit.name_placeholder')}
                onChange={handleInputChange}
                value={inputs.name}
                required
              />
            </Form.Field>
            {renderChannelTip(inputs.type)}

            {/* Azure OpenAI specific fields */}
            {inputs.type === 3 && (
              <>
                <Message>
                  注意，<strong>模型部署名称必须和模型名称保持一致</strong>
                  ，因为 One API 会把请求体中的 model
                  参数替换为你的部署名称（模型名称中的点会被剔除），
                  <a
                    target='_blank'
                    href='https://github.com/songquanpeng/one-api/issues/133?notification_referrer_id=NT_kwDOAmJSYrM2NjIwMzI3NDgyOjM5OTk4MDUw#issuecomment-1571602271'
                  >
                    图片演示
                  </a>
                  。
                </Message>
                <Form.Field>
                  <Form.Input
                    label='AZURE_OPENAI_ENDPOINT'
                    name='base_url'
                    placeholder='请输入 AZURE_OPENAI_ENDPOINT，例如：https://docs-test-001.openai.azure.com'
                    onChange={handleInputChange}
                    value={inputs.base_url}
                    autoComplete='new-password'
                  />
                </Form.Field>
                <Form.Field>
                  <Form.Input
                    label='默认 API 版本'
                    name='other'
                    placeholder='请输入默认 API 版本，例如：2024-03-01-preview，该配置可以被实际的请求查询参数所覆盖'
                    onChange={handleInputChange}
                    value={inputs.other}
                    autoComplete='new-password'
                  />
                </Form.Field>
              </>
            )}

            {/* Custom base URL field */}
            {inputs.type === 8 && (
              <Form.Field>
                <Form.Input
                    required
                    label={t('channel.edit.proxy_url')}
                    name='base_url'
                    placeholder={t('channel.edit.proxy_url_placeholder')}
                    onChange={handleInputChange}
                    value={inputs.base_url}
                    autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 50 && (
                <Form.Field>
                  <Form.Input
                      required
                  label={t('channel.edit.base_url')}
                  name='base_url'
                  placeholder={t('channel.edit.base_url_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.base_url}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}

            {inputs.type === 18 && (
              <Form.Field>
                <Form.Input
                  label={t('channel.edit.spark_version')}
                  name='other'
                  placeholder={t('channel.edit.spark_version_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.other}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 21 && (
              <Form.Field>
                <Form.Input
                  label={t('channel.edit.knowledge_id')}
                  name='other'
                  placeholder={t('channel.edit.knowledge_id_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.other}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 17 && (
              <Form.Field>
                <Form.Input
                  label={t('channel.edit.plugin_param')}
                  name='other'
                  placeholder={t('channel.edit.plugin_param_placeholder')}
                  onChange={handleInputChange}
                  value={inputs.other}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            {inputs.type === 34 && (
              <Message>{t('channel.edit.coze_notice')}</Message>
            )}
            {inputs.type === 40 && (
              <Message>
                {t('channel.edit.douban_notice')}
                <a
                  target='_blank'
                  href='https://console.volcengine.com/ark/region:ark+cn-beijing/endpoint'
                >
                  {t('channel.edit.douban_notice_link')}
                </a>
                {t('channel.edit.douban_notice_2')}
              </Message>
            )}
            {inputs.type === 33 && (
              <Form.Field>
                <Form.Input
                  label='Region'
                  name='region'
                  required
                  placeholder={t('channel.edit.aws_region_placeholder')}
                  onChange={handleConfigChange}
                  value={config.region}
                  autoComplete=''
                />
                <Form.Input
                  label='AK'
                  name='ak'
                  required
                  placeholder={t('channel.edit.aws_ak_placeholder')}
                  onChange={handleConfigChange}
                  value={config.ak}
                  autoComplete=''
                />
                <Form.Input
                  label='SK'
                  name='sk'
                  required
                  placeholder={t('channel.edit.aws_sk_placeholder')}
                  onChange={handleConfigChange}
                  value={config.sk}
                  autoComplete=''
                />
              </Form.Field>
            )}
            {inputs.type === 42 && (
              <Form.Field>
                <Form.Input
                  label='Region'
                  name='region'
                  required
                  placeholder={t('channel.edit.vertex_region_placeholder')}
                  onChange={handleConfigChange}
                  value={config.region}
                  autoComplete=''
                />
                <Form.Input
                  label={t('channel.edit.vertex_project_id')}
                  name='vertex_ai_project_id'
                  required
                  placeholder={t('channel.edit.vertex_project_id_placeholder')}
                  onChange={handleConfigChange}
                  value={config.vertex_ai_project_id}
                  autoComplete=''
                />
                <Form.Input
                  label={t('channel.edit.vertex_credentials')}
                  name='vertex_ai_adc'
                  required
                  placeholder={t('channel.edit.vertex_credentials_placeholder')}
                  onChange={handleConfigChange}
                  value={config.vertex_ai_adc}
                  autoComplete=''
                />
              </Form.Field>
            )}
            {inputs.type === 34 && (
              <Form.Input
                label={t('channel.edit.user_id')}
                name='user_id'
                required
                placeholder={t('channel.edit.user_id_placeholder')}
                onChange={handleConfigChange}
                value={config.user_id}
                autoComplete=''
              />
            )}
            {inputs.type !== 33 &&
              inputs.type !== 42 &&
              (batch ? (
                <Form.Field>
                  <Form.TextArea
                    label={t('channel.edit.key')}
                    name='key'
                    required
                    placeholder={t('channel.edit.batch_placeholder')}
                    onChange={handleInputChange}
                    value={inputs.key}
                    style={{
                      minHeight: 150,
                      fontFamily: 'JetBrains Mono, Consolas',
                    }}
                    autoComplete='new-password'
                  />
                </Form.Field>
              ) : (
                <Form.Field>
                  <Form.Input
                    label={t('channel.edit.key')}
                    name='key'
                    required
                    placeholder={type2secretPrompt(inputs.type, t)}
                    onChange={handleInputChange}
                    value={inputs.key}
                    autoComplete='new-password'
                  />
                </Form.Field>
              ))}
            {inputs.type === 37 && (
              <Form.Field>
                <Form.Input
                  label='Account ID'
                  name='user_id'
                  required
                  placeholder={
                    '请输入 Account ID，例如：d8d7c61dbc334c32d3ced580e4bf42b4'
                  }
                  onChange={handleConfigChange}
                  value={config.user_id}
                  autoComplete=''
                />
              </Form.Field>
            )}
            {inputs.type === 33 || inputs.type === 18 || inputs.type === 46 ? (
              <Message warning>{t('channel.edit.headers_unsupported')}</Message>
            ) : (
              <Form.Field>
                <label>{t('channel.edit.headers')}</label>
                <Message size='small'>{t('channel.edit.headers_prompt')}</Message>
                {inputs.type === 52 && (
                  <Message size='small' info>
                    {t('channel.edit.opencode_session_hint')}
                  </Message>
                )}
                {headerRows.map((row, index) => (
                  <Form.Group key={index} widths='equal' style={{ marginBottom: '8px' }}>
                    <Form.Input
                      placeholder={t('channel.edit.headers_key_placeholder')}
                      value={row.key}
                      error={row.key.trim() === '' && row.value !== ''}
                      onChange={(e, { value }) => updateHeaderRow(index, 'key', value)}
                      autoComplete='new-password'
                    />
                    <Form.Input
                      placeholder={t('channel.edit.headers_value_placeholder')}
                      value={row.value}
                      onChange={(e, { value }) => updateHeaderRow(index, 'value', value)}
                      autoComplete='new-password'
                    />
                    <Button
                      type='button'
                      basic
                      color='red'
                      onClick={() => removeHeaderRow(index)}
                    >
                      {t('channel.edit.remove_header')}
                    </Button>
                  </Form.Group>
                ))}
                <Button type='button' onClick={addHeaderRow}>
                  {t('channel.edit.add_header')}
                </Button>
              </Form.Field>
            )}
            {inputs.type !== 43 && (
              <Divider horizontal>{t('channel.edit.section_models')}</Divider>
            )}
            {inputs.type !== 43 && (
              <Form.Field>
                <Button
                  type='button'
                  loading={fetchModelsLoading}
                  disabled={
                    fetchModelsLoading ||
                    batch ||
                    (inputs.key === '' && !isEdit && !keylessProvider)
                  }
                  onClick={fetchUpstreamModels}
                >
                  {t('channel.edit.fetch_models')}
                </Button>
              </Form.Field>
            )}
            {inputs.type !== 43 && (
              <Form.Field>
                <Form.Dropdown
                  label={t('channel.edit.models')}
                  placeholder={t('channel.edit.models_placeholder')}
                  name='models'
                  required
                  fluid
                  multiple
                  search
                  onLabelClick={(e, { value }) => {
                    copy(value).then();
                  }}
                  selection
                  onChange={handleInputChange}
                  value={inputs.models}
                  autoComplete='new-password'
                  options={modelOptions}
                />
              </Form.Field>
            )}
            {inputs.type !== 43 && (
              <div style={{ lineHeight: '40px', marginBottom: '12px' }}>
                <Button
                  type={'button'}
                  disabled={basicModels.length === 0}
                  onClick={() => {
                    handleInputChange(null, {
                      name: 'models',
                      value: basicModels,
                    });
                  }}
                >
                  {t('channel.edit.buttons.fill_models')}
                </Button>
                <Button
                  type={'button'}
                  onClick={() => {
                    handleInputChange(null, {
                      name: 'models',
                      value: fullModels,
                    });
                  }}
                >
                  {t('channel.edit.buttons.fill_all')}
                </Button>
                <Button
                  type={'button'}
                  onClick={() => {
                    handleInputChange(null, { name: 'models', value: [] });
                  }}
                >
                  {t('channel.edit.buttons.clear')}
                </Button>
                <Input
                  action={
                    <Button type={'button'} onClick={addCustomModel}>
                      {t('channel.edit.buttons.add_custom')}
                    </Button>
                  }
                  placeholder={t('channel.edit.buttons.custom_placeholder')}
                  value={customModel}
                  onChange={(e, { value }) => {
                    setCustomModel(value);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      addCustomModel();
                      e.preventDefault();
                    }
                  }}
                />
              </div>
            )}
            {inputs.type !== 43 && (
              <>
                <Form.Field>
                  <Form.TextArea
                    label={t('channel.edit.model_mapping')}
                    placeholder={`${t(
                      'channel.edit.model_mapping_placeholder'
                    )}\n${JSON.stringify(MODEL_MAPPING_EXAMPLE, null, 2)}`}
                    name='model_mapping'
                    onChange={handleInputChange}
                    value={inputs.model_mapping}
                    style={{
                      minHeight: 150,
                      fontFamily: 'JetBrains Mono, Consolas',
                    }}
                    autoComplete='new-password'
                  />
                </Form.Field>
                <Form.Field>
                  <Form.TextArea
                    label={t('channel.edit.system_prompt')}
                    placeholder={t('channel.edit.system_prompt_placeholder')}
                    name='system_prompt'
                    onChange={handleInputChange}
                    value={inputs.system_prompt}
                    style={{
                      minHeight: 150,
                      fontFamily: 'JetBrains Mono, Consolas',
                    }}
                    autoComplete='new-password'
                  />
                </Form.Field>
              </>
            )}
            <Divider horizontal>{t('channel.edit.section_other')}</Divider>
            <Form.Field>
              <Form.Dropdown
                label={t('channel.edit.group')}
                placeholder={t('channel.edit.group_placeholder')}
                name='groups'
                required
                fluid
                multiple
                selection
                allowAdditions
                additionLabel={t('channel.edit.group_addition')}
                onChange={handleInputChange}
                value={inputs.groups}
                autoComplete='new-password'
                options={groupOptions}
              />
            </Form.Field>
            {inputs.type !== 33 && !isEdit && (
              <Form.Checkbox
                checked={batch}
                label={t('channel.edit.batch')}
                name='batch'
                onChange={() => setBatch(!batch)}
              />
            )}
            {inputs.type !== 3 &&
              inputs.type !== 33 &&
              inputs.type !== 8 &&
                inputs.type !== 50 &&
              inputs.type !== 22 && (
                <Form.Field>
                  <Form.Input
                      label={t('channel.edit.proxy_url')}
                    name='base_url'
                      placeholder={t('channel.edit.proxy_url_placeholder')}
                    onChange={handleInputChange}
                    value={inputs.base_url}
                    autoComplete='new-password'
                  />
                </Form.Field>
              )}
            {inputs.type === 22 && (
              <Form.Field>
                <Form.Input
                  label='私有部署地址'
                  name='base_url'
                  placeholder={
                    '请输入私有部署地址，格式为：https://fastgpt.run/api/openapi'
                  }
                  onChange={handleInputChange}
                  value={inputs.base_url}
                  autoComplete='new-password'
                />
              </Form.Field>
            )}
            <Button type='button' onClick={handleCancel}>
              {t('channel.edit.buttons.cancel')}
            </Button>
            <Button
              type={isEdit ? 'button' : 'submit'}
              positive
              onClick={submit}
            >
              {t('channel.edit.buttons.submit')}
            </Button>
          </Form>
          <Modal
            open={fetchedModels !== null}
            onClose={() => setFetchedModels(null)}
            size='small'
          >
            <Modal.Header>{t('channel.edit.fetch_models_title')}</Modal.Header>
            <Modal.Content scrolling>
              <div style={{ marginBottom: '12px' }}>
                <Button
                  type='button'
                  onClick={() => setFetchedModelSelection(fetchedModels || [])}
                >
                  {t('channel.edit.fetch_models_select_all')}
                </Button>
                <Button
                  type='button'
                  onClick={invertVisibleFetchedModels}
                >
                  {t('channel.edit.fetch_models_invert')}
                </Button>
                <Button type='button' onClick={() => setFetchedModelSelection([])}>
                  {t('channel.edit.fetch_models_clear')}
                </Button>
                <span style={{ marginLeft: '12px' }}>
                  {fetchedModelSelection.length} / {(fetchedModels || []).length}
                </span>
              </div>
              <Form>
                <Form.Input
                  icon='search'
                  placeholder={t('channel.edit.fetch_models_search_placeholder')}
                  value={fetchedModelKeyword}
                  onChange={(e, { value }) => setFetchedModelKeyword(value)}
                  autoComplete='new-password'
                />
                {(fetchedModels || [])
                  .filter((model) =>
                    model.toLowerCase().includes(fetchedModelKeyword.trim().toLowerCase())
                  )
                  .map((model) => (
                    <Form.Checkbox
                      key={model}
                      label={
                        inputs.models.includes(model)
                          ? `${model} · ${t('channel.edit.fetch_models_already_added')}`
                          : model
                      }
                      checked={fetchedModelSelection.includes(model)}
                      onChange={() => toggleFetchedModel(model)}
                    />
                  ))}
              </Form>
            </Modal.Content>
            <Modal.Actions>
              <Button type='button' onClick={() => setFetchedModels(null)}>
                {t('channel.edit.buttons.cancel')}
              </Button>
              <Button type='button' positive onClick={confirmFetchedModels}>
                {t('channel.edit.fetch_models_confirm')}
              </Button>
            </Modal.Actions>
          </Modal>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditChannel;
